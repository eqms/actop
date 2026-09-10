package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	// httpTimeout bounds the whole request. The check runs off the update
	// loop, but a hung connection must not keep a goroutine alive for the
	// lifetime of the program.
	httpTimeout = 5 * time.Second
	// maxBody caps the response, mirroring the caps ExecClient.run applies
	// to subprocess output.
	maxBody = 1 << 20 // 1 MiB
	// ContainerScript is Apple's own updater, installed by the signed
	// package. It refuses to run while the container service is up, which
	// is why the hint points at `actop update-container` instead.
	ContainerScript = "/usr/local/bin/update-container.sh"
	releasesPage    = "https://github.com/apple/container/releases"
)

// source binds a component name to its GitHub repository. A slice rather
// than a map: the render order must not change between starts. The repos are
// constants -- a configurable URL would turn the checker into a request
// forwarder.
type source struct{ name, repo string }

var sources = []source{
	{"container", "apple/container"},
	{"actop", "eqms/actop"},
}

// Component is a single outdated piece of software plus the command that
// updates it. actop never runs that command.
type Component struct {
	Name    string // "container" | "actop"
	Current string // canonical, e.g. "1.1.0"
	Latest  string // canonical, e.g. "1.2.0"
	Hint    string // the command the user should run
}

// Result carries only outdated components; up-to-date ones are dropped.
type Result struct {
	Components []Component
	CheckedAt  time.Time
}

// Count is nil-safe and zero-value-safe, so the UI can call it before any
// check has completed.
func (r Result) Count() int { return len(r.Components) }

// Checker performs the release lookup. The zero value is usable; the fields
// exist so tests can pin the clock, the cache location and the endpoint.
type Checker struct {
	HTTP     *http.Client
	BaseURL  string
	CacheDir string
	Now      func() time.Time
}

func (c *Checker) baseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return c.BaseURL
}

func (c *Checker) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: httpTimeout}
}

func (c *Checker) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}

func (c *Checker) cacheDir() string {
	if c.CacheDir != "" {
		return c.CacheDir
	}
	return defaultCacheDir()
}

// Run compares the locally installed versions against the latest releases.
// It never returns an error: a failed lookup yields an empty Result, because
// a monitoring tool must not report on its own update checker.
func (c *Checker) Run(ctx context.Context, local map[string]string) Result {
	now := c.now()
	res := Result{CheckedAt: now}

	// Only components with a parseable local version are worth checking; a
	// development build ("dev") drops out here without a special case.
	wanted := make([]source, 0, len(sources))
	current := make(map[string]Version, len(sources))
	for _, s := range sources {
		v, ok := Parse(local[s.name])
		if !ok {
			continue
		}
		wanted = append(wanted, s)
		current[s.name] = v
	}
	if len(wanted) == 0 {
		return res
	}

	latest, fromCache := c.cachedLatest(wanted, now)
	if !fromCache {
		latest = c.fetchAll(ctx, wanted)
		// Never cache a run in which nothing could be fetched: one outage
		// would otherwise silence the check for a full day.
		if len(latest) > 0 {
			_ = writeCache(c.cacheDir(), cacheEntry{CheckedAt: now, Latest: latest})
		}
	}

	for _, s := range wanted {
		remote, ok := Parse(latest[s.name])
		if !ok {
			continue
		}
		cur := current[s.name]
		if !cur.Less(remote) {
			continue
		}
		res.Components = append(res.Components, Component{
			Name:    s.name,
			Current: cur.String(),
			Latest:  remote.String(),
			Hint:    hintFor(s.name),
		})
	}
	return res
}

// cachedLatest returns the stored versions if the entry is fresh and covers
// every wanted component; a partial cache is treated as a miss so a newly
// added source is picked up on the next start.
func (c *Checker) cachedLatest(wanted []source, now time.Time) (map[string]string, bool) {
	e, ok := readCache(c.cacheDir())
	if !ok || !e.fresh(now) {
		return nil, false
	}
	for _, s := range wanted {
		if _, has := e.Latest[s.name]; !has {
			return nil, false
		}
	}
	return e.Latest, true
}

// fetchAll queries every wanted source. A component that fails is simply
// absent from the result; the others still count.
func (c *Checker) fetchAll(ctx context.Context, wanted []source) map[string]string {
	out := make(map[string]string, len(wanted))
	for _, s := range wanted {
		if tag, ok := c.fetchLatest(ctx, s.repo); ok {
			out[s.name] = tag
		}
	}
	return out
}

// fetchLatest reads tag_name from the GitHub releases API. /releases/latest
// excludes drafts and prereleases, so no client-side filtering is needed.
func (c *Checker) fetchLatest(ctx context.Context, repo string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL()+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// GitHub rejects API requests without a User-Agent.
	req.Header.Set("User-Agent", "actop")

	resp, err := c.client().Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", false
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	// Validate here, so an unparseable tag is never cached or rendered.
	v, ok := Parse(payload.TagName)
	if !ok {
		return "", false
	}
	return v.String(), true
}

// Latest looks up the newest release of one component, bypassing the cache.
// `actop update-container` decides from it whether to stop the container
// service at all, so a day-old answer is not good enough there.
func (c *Checker) Latest(ctx context.Context, name string) (string, bool) {
	for _, s := range sources {
		if s.name == name {
			return c.fetchLatest(ctx, s.repo)
		}
	}
	return "", false
}

// hintFor returns the command that updates a component. For Apple Container
// that is actop's wrapper around the updater from the signed package, when
// that updater is present -- pointing at a path that does not exist would be
// worse than pointing at the release page.
func hintFor(name string) string {
	switch name {
	case "container":
		if _, err := os.Stat(ContainerScript); err == nil {
			return "actop update-container"
		}
		return releasesPage
	case "actop":
		return "brew upgrade actop"
	}
	return ""
}
