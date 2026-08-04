# actop Update Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On start, actop reports once a day whether a newer Apple Container or actop release exists and which command installs it — notification only, actop never runs an installer.

**Architecture:** A new leaf package `update/` (stdlib only, no UI knowledge) fetches the latest release tag from the GitHub API, compares it against the locally installed versions and caches the result for 24 h. The UI consumes it through a single `tea.Cmd`/`tea.Msg` pair fired from `App.Init`, so `model.Store` and `metrics.Engine` stay lock-free. Rendering happens in two places: a badge in the header and an `UPDATES` block in the system tab.

**Tech Stack:** Go (stdlib `net/http`, `encoding/json`, `os`), Bubble Tea v2 (`charm.land/bubbletea/v2`), existing `cli`/`config`/`ui` packages.

**Spec:** `docs/superpowers/specs/2026-08-04-update-check-design.md`

## Global Constraints

- **No new module dependencies.** `go.mod` keeps exactly its current five direct requires. The semver parser is hand-written for this reason.
- **Go stdlib only in `update/`.** The package must not import `cli`, `ui`, `config`, `model` or `metrics` — it is a leaf, like `metrics/`.
- **Every rendered string that originates outside actop goes through `safe()`** (or through `pad()`/`displayWidth()`, which apply it).
- **`update.Checker.Run` never returns an error.** Network, HTTP, JSON and cache failures all yield an empty `Result`.
- **Bounded I/O:** HTTP client timeout 5 s, response body read through `io.LimitReader` at 1 MiB (`maxBody = 1 << 20`).
- **No credentials, no user-controlled URLs.** Unauthenticated requests; repos are package constants; `Checker.BaseURL` exists only for tests.
- **Comments and code in English**, user-facing German only in `RELEASE_NOTES.md`/README.
- **Tests never touch the network.** `httptest.Server` plus `Checker.BaseURL`; `Checker.Now` for clock control.
- **Commit prefixes:** `[ADD]` new features, `[CHG]` modifications, `[FIX]` bug fixes.
- **Verification per task:** `go test -race ./...` (`make test`) must pass; `make lint` must pass before the final task's commit.
- Every commit message ends with the two trailer lines used in this repo:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01U5fQtZ1NKrhBYeRTz62Uzw
  ```
  They are omitted from the command blocks below for brevity — add them to each.

## File Structure

| File | Responsibility |
|------|----------------|
| `update/semver.go` (create) | `Version` type, `Parse`, `Less`, `String`. Also the input filter for remote tags. |
| `update/semver_test.go` (create) | Parser and comparison tests, including hostile tags. |
| `update/cache.go` (create) | Read/write `$XDG_CACHE_HOME/actop/update.json`, TTL logic, `defaultCacheDir`. |
| `update/cache_test.go` (create) | Miss/hit/expiry/future-clock/unwritable-dir tests. |
| `update/check.go` (create) | `Checker`, `Component`, `Result`, HTTP fetch, hint selection. |
| `update/check_test.go` (create) | `httptest`-driven behaviour and failure modes. |
| `cli/client.go` (modify) | Add `Version(ctx)` to the `Client` interface. |
| `cli/exec.go` (modify) | `ExecClient.Version` via `container --version`. |
| `cli/mock.go` (modify) | `MockClient.VersionValue` + `Version`. |
| `cli/exec_test.go` (modify) | `Version` against the fake-binary fixture. |
| `config/config.go` (modify) | `UpdateCheck *bool`, `UpdateCheckEnabled()`. |
| `config/config_test.go` (modify) | Default/opt-out/env-override tests. |
| `ui/messages.go` (modify) | `updatesMsg`. |
| `ui/commands.go` (modify) | `checkUpdatesCmd`. |
| `ui/app.go` (modify) | `updates`/`checker` fields, `Init`, `Update`, `header`, `updateBadge`. |
| `ui/system.go` (modify) | `UPDATES` block. |
| `ui/app_test.go` (modify) | Wiring, badge and suppression tests. |
| `ui/program_test.go` (modify) | Env guard so the smoke test stays offline. |
| `VERSION`, `RELEASE_NOTES.md`, `README.md` (modify) | Release paperwork. |

---

### Task 1: Semver parsing and comparison

**Files:**
- Create: `update/semver.go`
- Test: `update/semver_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `update.Version` struct with fields `Major, Minor, Patch int`; `update.Parse(s string) (Version, bool)`; `(Version) Less(o Version) bool`; `(Version) String() string`. Tasks 2, 3 and 6 depend on these exact names.

- [ ] **Step 1: Write the failing test**

Create `update/semver_test.go`:

```go
package update

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Version
		ok   bool
	}{
		{"1.2.0", Version{1, 2, 0}, true},
		{"v1.2.0", Version{1, 2, 0}, true},
		{"v0.6.1", Version{0, 6, 1}, true},
		{"1.2", Version{}, false},
		{"1.2.0.1", Version{}, false},
		{"v1.2.0-beta.1", Version{}, false},
		{"1.2.x", Version{}, false},
		{"", Version{}, false},
		{"v-1.2.0", Version{}, false},
		{" 1.2.0", Version{}, false},
		// A release tag is attacker-controllable in the general case; the
		// parser is the first filter that keeps terminal escapes out of the
		// render path.
		{"1.2.0\x1b]52;c;cGF5bG9hZA==\x07", Version{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok {
			t.Errorf("Parse(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// Numeric comparison, not lexicographic: "1.10.0" must outrank "1.9.0".
func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"1.9.0", "1.10.0", true},
		{"1.2.0", "1.2.1", true},
		{"1.2.0", "1.2.0", false},
		{"2.0.0", "1.9.9", false},
		{"0.6.1", "0.7.0", true},
	}
	for _, c := range cases {
		a, _ := Parse(c.a)
		b, _ := Parse(c.b)
		if got := a.Less(b); got != c.want {
			t.Errorf("%s.Less(%s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// The canonical form is what gets rendered — never the bytes the remote sent.
func TestString(t *testing.T) {
	v, _ := Parse("v1.2.0")
	if v.String() != "1.2.0" {
		t.Errorf("String() = %q, want %q", v.String(), "1.2.0")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./update/ -run 'TestParse|TestLess|TestString' -v`
Expected: FAIL — the package does not exist yet (`no Go files in .../update`).

- [ ] **Step 3: Write minimal implementation**

Create `update/semver.go`:

```go
// Package update checks whether newer releases of Apple Container and of
// actop are available. It only reports; it never installs anything.
//
// It is a leaf package: stdlib only, no knowledge of the CLI or the UI.
package update

import (
	"strconv"
	"strings"
)

// Version is a three-component release version. Prerelease and build
// metadata are deliberately unsupported: the GitHub /releases/latest
// endpoint never points at a prerelease, so anything carrying a suffix is
// not a release actop should advertise.
type Version struct{ Major, Minor, Patch int }

// Parse accepts "1.2.0" and "v1.2.0" and nothing else. It is the primary
// input filter for remote release tags: a tag that does not match this
// grammar never reaches the render path, so terminal escape sequences
// smuggled into a tag name are rejected here rather than sanitized later.
func Parse(s string) (Version, bool) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var out [3]int
	for i, p := range parts {
		// strconv.Atoi accepts a leading sign; a version component must be
		// plain digits, so reject anything else outright.
		if p == "" || strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return Version{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		out[i] = n
	}
	return Version{Major: out[0], Minor: out[1], Patch: out[2]}, true
}

// Less reports whether v precedes o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// String returns the canonical form without a "v" prefix.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./update/ -race -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add update/semver.go update/semver_test.go
git commit -m "[ADD] update: semver parsing and comparison"
```

---

### Task 2: On-disk cache

**Files:**
- Create: `update/cache.go`
- Test: `update/cache_test.go`

**Interfaces:**
- Consumes: nothing from Task 1 (independent), but lives in the same package.
- Produces: `cacheTTL` constant; `type cacheEntry struct { CheckedAt time.Time; Latest map[string]string }`; `cachePath(dir string) string`; `readCache(dir string) (cacheEntry, bool)`; `writeCache(dir string, e cacheEntry) error`. Task 3 calls all of these.

- [ ] **Step 1: Write the failing test**

Create `update/cache_test.go`:

```go
package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	in := cacheEntry{CheckedAt: now, Latest: map[string]string{"container": "1.2.0"}}
	if err := writeCache(dir, in); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	got, ok := readCache(dir)
	if !ok {
		t.Fatal("readCache reported a miss right after a write")
	}
	if got.Latest["container"] != "1.2.0" || !got.CheckedAt.Equal(now) {
		t.Errorf("round trip lost data: %+v", got)
	}
}

// The cache holds nothing secret, but there is no reason to be looser than
// the config file it sits next to.
func TestCacheFilePermissions(t *testing.T) {
	dir := t.TempDir()
	if err := writeCache(dir, cacheEntry{CheckedAt: time.Now()}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	fi, err := os.Stat(cachePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache mode = %o, want 600", perm)
	}
}

func TestCacheMissOnAbsentAndGarbage(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readCache(dir); ok {
		t.Error("absent file must be a miss")
	}
	if err := os.MkdirAll(filepath.Dir(cachePath(dir)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath(dir), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCache(dir); ok {
		t.Error("malformed JSON must be a miss")
	}
}

func TestCacheFresh(t *testing.T) {
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		aged  time.Duration
		fresh bool
	}{
		{"just written", 0, true},
		{"one hour old", time.Hour, true},
		{"just under a day", 23 * time.Hour, true},
		{"just over a day", 25 * time.Hour, false},
		// Clock moved backwards: pinning the cache until the far future
		// would silence the check for as long as the skew lasts.
		{"stamped in the future", -time.Hour, false},
	}
	for _, c := range cases {
		e := cacheEntry{CheckedAt: now.Add(-c.aged)}
		if got := e.fresh(now); got != c.fresh {
			t.Errorf("%s: fresh = %v, want %v", c.name, got, c.fresh)
		}
	}
}

// A read-only home must degrade to "no caching", never to a failed check.
func TestWriteCacheOnUnwritableDirReturnsError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := writeCache(dir, cacheEntry{CheckedAt: time.Now()}); err == nil {
		t.Error("writeCache into an unwritable dir must report an error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./update/ -run TestCache -v`
Expected: FAIL — `undefined: cacheEntry`, `undefined: writeCache`, `undefined: readCache`, `undefined: cachePath`.

- [ ] **Step 3: Write minimal implementation**

Create `update/cache.go`:

```go
package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cacheTTL bounds how often actop asks GitHub. Two requests per day is far
// below the 60/hour unauthenticated rate limit and keeps a long-running
// terminal from becoming a polling client of github.com.
const cacheTTL = 24 * time.Hour

// cacheEntry is the on-disk shape of $XDG_CACHE_HOME/actop/update.json. It
// stores only the fetched remote versions, never the local ones, so an
// upgrade between two starts takes effect immediately.
type cacheEntry struct {
	CheckedAt time.Time         `json:"checkedAt"`
	Latest    map[string]string `json:"latest"`
}

// fresh reports whether the entry may be used without refetching. A stamp in
// the future counts as stale rather than as valid for a very long time.
func (e cacheEntry) fresh(now time.Time) bool {
	if e.CheckedAt.After(now) {
		return false
	}
	return now.Sub(e.CheckedAt) < cacheTTL
}

// defaultCacheDir resolves the base directory, mirroring how config.Path
// resolves XDG_CONFIG_HOME. An empty result means "no caching possible".
func defaultCacheDir() string {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" {
		return base
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache")
}

func cachePath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "actop", "update.json")
}

// readCache returns the stored entry. A missing file, an unreadable file and
// malformed JSON are all reported as a plain miss.
func readCache(dir string) (cacheEntry, bool) {
	path := cachePath(dir)
	if path == "" {
		return cacheEntry{}, false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is derived from XDG_CACHE_HOME, not from remote input
	if err != nil {
		return cacheEntry{}, false
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return cacheEntry{}, false
	}
	return e, true
}

// writeCache persists the entry. Callers treat failure as non-fatal: without
// a cache the check simply runs once per start.
func writeCache(dir string, e cacheEntry) error {
	path := cachePath(dir)
	if path == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./update/ -race -v`
Expected: PASS (Task 1 tests plus 5 cache tests).

- [ ] **Step 5: Commit**

```bash
git add update/cache.go update/cache_test.go
git commit -m "[ADD] update: 24h on-disk cache for release checks"
```

---

### Task 3: The checker

**Files:**
- Create: `update/check.go`
- Test: `update/check_test.go`

**Interfaces:**
- Consumes: `Parse`, `Version.Less`, `Version.String` (Task 1); `cacheEntry`, `readCache`, `writeCache`, `defaultCacheDir`, `fresh` (Task 2).
- Produces: `update.Component{Name, Current, Latest, Hint string}`; `update.Result{Components []Component; CheckedAt time.Time}` with method `Count() int`; `update.Checker{HTTP *http.Client; BaseURL, CacheDir string; Now func() time.Time}` with method `Run(ctx context.Context, local map[string]string) Result`. Task 6 constructs a `Checker` and calls `Run`.

- [ ] **Step 1: Write the failing test**

Create `update/check_test.go`:

```go
package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

// releaseServer serves the GitHub /releases/latest shape for each repo and
// counts requests so cache behaviour can be asserted.
func releaseServer(t *testing.T, tags map[string]string, hits *int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt64(hits, 1)
		}
		for repo, tag := range tags {
			if r.URL.Path == "/repos/"+repo+"/releases/latest" {
				fmt.Fprintf(w, `{"tag_name":%q}`, tag)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testChecker(t *testing.T, srv *httptest.Server) *Checker {
	t.Helper()
	return &Checker{
		BaseURL:  srv.URL,
		CacheDir: t.TempDir(),
		Now:      func() time.Time { return testNow },
	}
}

func TestRunReportsOutdatedComponents(t *testing.T) {
	srv := releaseServer(t, map[string]string{
		"apple/container": "1.2.0",
		"eqms/actop":      "v0.7.0",
	}, nil)
	res := testChecker(t, srv).Run(context.Background(),
		map[string]string{"container": "1.1.0", "actop": "0.6.1"})

	if res.Count() != 2 {
		t.Fatalf("Count = %d, want 2; components = %+v", res.Count(), res.Components)
	}
	// Order is fixed so the system tab does not shuffle between starts.
	if res.Components[0].Name != "container" || res.Components[1].Name != "actop" {
		t.Errorf("unexpected order: %+v", res.Components)
	}
	c := res.Components[0]
	if c.Current != "1.1.0" || c.Latest != "1.2.0" {
		t.Errorf("container: current/latest = %s/%s, want 1.1.0/1.2.0", c.Current, c.Latest)
	}
	if c.Hint == "" {
		t.Error("a component without a hint tells the user nothing actionable")
	}
	if res.Components[1].Hint != "brew upgrade actop" {
		t.Errorf("actop hint = %q", res.Components[1].Hint)
	}
}

func TestRunSkipsUpToDateAndNewerLocals(t *testing.T) {
	srv := releaseServer(t, map[string]string{
		"apple/container": "1.2.0",
		"eqms/actop":      "0.7.0",
	}, nil)
	res := testChecker(t, srv).Run(context.Background(),
		map[string]string{"container": "1.2.0", "actop": "0.8.0"})
	if res.Count() != 0 {
		t.Errorf("nothing is outdated, got %+v", res.Components)
	}
}

// A development build has no meaningful version to compare against.
func TestRunSkipsUnparseableLocalVersion(t *testing.T) {
	srv := releaseServer(t, map[string]string{"apple/container": "1.2.0"}, nil)
	res := testChecker(t, srv).Run(context.Background(),
		map[string]string{"container": "1.1.0", "actop": "dev"})
	if res.Count() != 1 || res.Components[0].Name != "container" {
		t.Errorf("want only container reported, got %+v", res.Components)
	}
}

// A tag that is not plain semver is discarded rather than rendered: this is
// what keeps a hostile release name out of the terminal.
func TestRunRejectsMalformedTags(t *testing.T) {
	for _, tag := range []string{
		"latest",
		"v1.2.0-beta.1",
		"1.2.0\x1b]52;c;cGF5bG9hZA==\x07",
		"",
	} {
		srv := releaseServer(t, map[string]string{"apple/container": tag}, nil)
		res := testChecker(t, srv).Run(context.Background(),
			map[string]string{"container": "1.1.0"})
		if res.Count() != 0 {
			t.Errorf("tag %q must be discarded, got %+v", tag, res.Components)
		}
	}
}

// A monitoring tool must not report on its own update checker.
func TestRunSurvivesServerFailures(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }},
		{"403 rate limited", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }},
		{"malformed json", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"tag_name":`)
		}},
		{"oversized body", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"tag_name":"1.2.0","junk":"`)
			fmt.Fprint(w, strings.Repeat("x", 2<<20))
			fmt.Fprint(w, `"}`)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			res := testChecker(t, srv).Run(context.Background(),
				map[string]string{"container": "1.1.0"})
			if res.Count() != 0 {
				t.Errorf("want empty result, got %+v", res.Components)
			}
		})
	}
}

func TestRunSurvivesUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing is listening any more
	ck := &Checker{BaseURL: url, CacheDir: t.TempDir(), Now: func() time.Time { return testNow }}
	if res := ck.Run(context.Background(), map[string]string{"container": "1.1.0"}); res.Count() != 0 {
		t.Errorf("want empty result, got %+v", res.Components)
	}
}

func TestRunUsesCacheWithinTTL(t *testing.T) {
	var hits int64
	srv := releaseServer(t, map[string]string{
		"apple/container": "1.2.0",
		"eqms/actop":      "0.7.0",
	}, &hits)
	dir := t.TempDir()
	local := map[string]string{"container": "1.1.0", "actop": "0.6.1"}

	ck := &Checker{BaseURL: srv.URL, CacheDir: dir, Now: func() time.Time { return testNow }}
	if res := ck.Run(context.Background(), local); res.Count() != 2 {
		t.Fatalf("first run: Count = %d", res.Count())
	}
	first := atomic.LoadInt64(&hits)
	if first == 0 {
		t.Fatal("first run made no request")
	}

	// Same day: served from disk, no further requests.
	ck2 := &Checker{BaseURL: srv.URL, CacheDir: dir, Now: func() time.Time { return testNow.Add(time.Hour) }}
	if res := ck2.Run(context.Background(), local); res.Count() != 2 {
		t.Errorf("cached run: Count = %d, want 2", res.Count())
	}
	if got := atomic.LoadInt64(&hits); got != first {
		t.Errorf("cached run made %d extra requests", got-first)
	}

	// Two days later: refetched.
	ck3 := &Checker{BaseURL: srv.URL, CacheDir: dir, Now: func() time.Time { return testNow.Add(48 * time.Hour) }}
	if res := ck3.Run(context.Background(), local); res.Count() != 2 {
		t.Errorf("expired run: Count = %d, want 2", res.Count())
	}
	if atomic.LoadInt64(&hits) <= first {
		t.Error("expired cache must trigger a refetch")
	}
}

// A failed fetch must not be cached: otherwise one outage silences the check
// for a full day.
func TestRunDoesNotCacheFailures(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	dir := t.TempDir()
	local := map[string]string{"container": "1.1.0"}

	ck := &Checker{BaseURL: srv.URL, CacheDir: dir, Now: func() time.Time { return testNow }}
	ck.Run(context.Background(), local)
	after := atomic.LoadInt64(&hits)
	ck.Run(context.Background(), local)
	if atomic.LoadInt64(&hits) <= after {
		t.Error("a failed check must be retried on the next start, not cached")
	}
}

// The zero value must be usable: nil HTTP client, empty BaseURL, nil Now.
func TestZeroValueCheckerHasDefaults(t *testing.T) {
	var ck Checker
	if ck.baseURL() != defaultBaseURL {
		t.Errorf("baseURL() = %q, want %q", ck.baseURL(), defaultBaseURL)
	}
	if ck.client() == nil {
		t.Error("client() must never be nil")
	}
	if ck.now().IsZero() {
		t.Error("now() must return a real time")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./update/ -run 'TestRun|TestZeroValueChecker' -v`
Expected: FAIL — `undefined: Checker`, `undefined: defaultBaseURL`.

- [ ] **Step 3: Write minimal implementation**

Create `update/check.go`:

```go
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
	// updateScript is Apple's own updater, installed by the signed package.
	updateScript = "/usr/local/bin/update-container.sh"
	releasesPage = "https://github.com/apple/container/releases"
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

// hintFor returns the command that updates a component. For Apple Container
// that is the updater from the signed package when it is present -- pointing
// at a path that does not exist would be worse than pointing at the release
// page.
func hintFor(name string) string {
	switch name {
	case "container":
		if _, err := os.Stat(updateScript); err == nil {
			return updateScript
		}
		return releasesPage
	case "actop":
		return "brew upgrade actop"
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./update/ -race -v`
Expected: PASS (all `update` tests).

- [ ] **Step 5: Commit**

```bash
git add update/check.go update/check_test.go
git commit -m "[ADD] update: GitHub release checker with strict tag validation"
```

---

### Task 4: `cli.Client.Version`

**Files:**
- Modify: `cli/client.go` (interface), `cli/exec.go` (append method), `cli/mock.go`
- Test: `cli/exec_test.go` (append)

**Interfaces:**
- Consumes: `ExecClient.run` and the `fakeBinary` test helper, both existing.
- Produces: `Version(ctx context.Context) (string, error)` on `cli.Client`; `MockClient.VersionValue string` and `MockClient.VersionErr error`. Task 6 calls `client.Version`.

- [ ] **Step 1: Write the failing test**

Append to `cli/exec_test.go`:

```go
// The real CLI prints "container CLI version 1.2.0 (build: release, commit:
// 6e65319)"; only the version number is of interest.
func TestVersionExtractsNumber(t *testing.T) {
	c := fakeBinary(t, `echo "container CLI version 1.2.0 (build: release, commit: 6e65319)"`, 1024)
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "1.2.0" {
		t.Errorf("Version = %q, want %q", v, "1.2.0")
	}
}

func TestVersionRejectsUnrecognizedOutput(t *testing.T) {
	c := fakeBinary(t, `echo "something else entirely"`, 1024)
	if _, err := c.Version(context.Background()); err == nil {
		t.Error("output without a version number must be an error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestVersion -v`
Expected: FAIL — `c.Version undefined`.

- [ ] **Step 3: Write minimal implementation**

Add to the `Client` interface in `cli/client.go`, directly below the `SystemStatus` entry:

```go
	// Version reports the container CLI version, e.g. "1.2.0". Unlike
	// SystemStatus it also answers while the API server is stopped.
	Version(ctx context.Context) (string, error)
```

Append to `cli/exec.go` (and add `"regexp"` to its import block):

```go
// versionPattern pulls the three-component version out of the CLI banner,
// which also carries build and commit information.
var versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Version returns the container CLI version. apiServerVersion from
// `system status` is deliberately not reused: it is empty while the API
// server is stopped, which is exactly the state a user is in right after
// `container system stop` -- immediately before updating.
func (c *ExecClient) Version(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "--version")
	if err != nil {
		return "", err
	}
	v := versionPattern.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("parse container version: no version in %q", string(out))
	}
	return v, nil
}
```

Add to the `MockClient` struct in `cli/mock.go`, below `ActionFn`:

```go
	// VersionValue is returned by Version; empty means "1.0.0".
	VersionValue string
	VersionErr   error
```

and append the method next to the other recorded getters:

```go
func (m *MockClient) Version(ctx context.Context) (string, error) {
	m.record("Version", "")
	if m.VersionErr != nil {
		return "", m.VersionErr
	}
	if m.VersionValue != "" {
		return m.VersionValue, nil
	}
	return "1.0.0", nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -race`
Expected: PASS everywhere. `MockClient` must still satisfy `Client`; if any other implementation of the interface exists the compiler will say so here.

- [ ] **Step 5: Commit**

```bash
git add cli/client.go cli/exec.go cli/mock.go cli/exec_test.go
git commit -m "[ADD] cli: Version method for the container CLI"
```

---

### Task 5: Config key and env opt-out

**Files:**
- Modify: `config/config.go`
- Test: `config/config_test.go` (append)

**Interfaces:**
- Consumes: existing `Config`/`Default`.
- Produces: field `UpdateCheck *bool` with TOML key `updateCheck`; method `(Config) UpdateCheckEnabled() bool`; exported constant `EnvNoUpdateCheck = "ACTOP_NO_UPDATE_CHECK"`. Task 6 calls `UpdateCheckEnabled`.

- [ ] **Step 1: Write the failing test**

Append to `config/config_test.go`:

```go
// The check is on unless the user turns it off; an absent key must not be
// indistinguishable from an explicit false.
func TestUpdateCheckEnabled(t *testing.T) {
	no, yes := false, true
	cases := []struct {
		name string
		cfg  Config
		env  string
		want bool
	}{
		{"default", Default(), "", true},
		{"absent key", Config{}, "", true},
		{"explicit true", Config{UpdateCheck: &yes}, "", true},
		{"explicit false", Config{UpdateCheck: &no}, "", false},
		{"env overrides enabled config", Config{UpdateCheck: &yes}, "1", false},
		{"env=0 does not disable", Config{UpdateCheck: &yes}, "0", true},
		{"env with any value disables", Default(), "yes", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(EnvNoUpdateCheck, c.env)
			if got := c.cfg.UpdateCheckEnabled(); got != c.want {
				t.Errorf("UpdateCheckEnabled = %v, want %v", got, c.want)
			}
		})
	}
}

func TestUpdateCheckParsedFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "actop"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "actop", "config.toml")
	if err := os.WriteFile(path, []byte("updateCheck = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Setenv(EnvNoUpdateCheck, "")
	if cfg.UpdateCheckEnabled() {
		t.Error("updateCheck = false in the file must disable the check")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestUpdateCheck -v`
Expected: FAIL — `undefined: EnvNoUpdateCheck`, `unknown field UpdateCheck`.

- [ ] **Step 3: Write minimal implementation**

In `config/config.go`, add the field to `Config`:

```go
	// UpdateCheck is a pointer so an explicit `updateCheck = false` stays
	// distinguishable from an absent key, which must default to true.
	UpdateCheck *bool `toml:"updateCheck"`
```

`Default()` stays unchanged — a nil pointer already means "enabled".

Append below `Load`:

```go
// EnvNoUpdateCheck disables the release check for a single run, without
// touching the config file.
const EnvNoUpdateCheck = "ACTOP_NO_UPDATE_CHECK"

// UpdateCheckEnabled reports whether actop may contact the GitHub API on
// start. The environment wins over the file so the check can be suppressed
// per run; an absent setting means enabled.
func (c Config) UpdateCheckEnabled() bool {
	if v := os.Getenv(EnvNoUpdateCheck); v != "" && v != "0" {
		return false
	}
	if c.UpdateCheck != nil {
		return *c.UpdateCheck
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "[ADD] config: updateCheck key and ACTOP_NO_UPDATE_CHECK opt-out"
```

---

### Task 6: Wire the check into the update loop

**Files:**
- Modify: `ui/messages.go`, `ui/commands.go`, `ui/app.go`, `ui/program_test.go`
- Test: `ui/app_test.go` (append)

**Interfaces:**
- Consumes: `update.Checker.Run`, `update.Result` (Task 3); `cli.Client.Version` (Task 4); `config.Config.UpdateCheckEnabled`, `config.EnvNoUpdateCheck` (Task 5).
- Produces: `ui.updatesMsg{result update.Result}`; `ui.checkUpdatesCmd(c cli.Client, cfg config.Config, version string, ck *update.Checker) tea.Cmd`; `App.updates update.Result` and `App.checker *update.Checker`. Tasks 7 and 8 read `App.updates`.

- [ ] **Step 1: Write the failing test**

Append to `ui/app_test.go`:

```go
func TestUpdatesMsgStoresResult(t *testing.T) {
	app, _ := testApp()
	app.Update(updatesMsg{result: update.Result{
		Components: []update.Component{
			{Name: "container", Current: "1.1.0", Latest: "1.2.0", Hint: "/usr/local/bin/update-container.sh"},
		},
	}})
	if app.updates.Count() != 1 {
		t.Fatalf("updates.Count = %d, want 1", app.updates.Count())
	}
}

// Opting out must prevent the command from existing at all -- not merely
// from rendering its result.
func TestCheckUpdatesCmdRespectsOptOut(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "1")
	mock := &cli.MockClient{}
	if cmd := checkUpdatesCmd(mock, config.Default(), "0.6.1", nil); cmd != nil {
		t.Fatal("opt-out must yield a nil command")
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("opt-out must not touch the client, got %+v", mock.CallList())
	}
}

// The command must ask the CLI for the installed container version and hand
// both versions to the checker.
func TestCheckUpdatesCmdReportsBothComponents(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apple/container/releases/latest":
			fmt.Fprint(w, `{"tag_name":"1.2.0"}`)
		case "/repos/eqms/actop/releases/latest":
			fmt.Fprint(w, `{"tag_name":"v0.7.0"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	mock := &cli.MockClient{VersionValue: "1.1.0"}
	ck := &update.Checker{BaseURL: srv.URL, CacheDir: t.TempDir()}
	cmd := checkUpdatesCmd(mock, config.Default(), "0.6.1", ck)
	if cmd == nil {
		t.Fatal("want a command")
	}
	msg, ok := cmd().(updatesMsg)
	if !ok {
		t.Fatalf("command returned %T, want updatesMsg", cmd())
	}
	if msg.result.Count() != 2 {
		t.Errorf("Count = %d, want 2: %+v", msg.result.Count(), msg.result.Components)
	}
	var sawVersion bool
	for _, c := range mock.CallList() {
		if c.Method == "Version" {
			sawVersion = true
		}
	}
	if !sawVersion {
		t.Error("the command must query the container CLI version")
	}
}

// A container CLI that is missing or broken must not suppress the actop
// check -- the two components are independent.
func TestCheckUpdatesCmdSurvivesVersionError(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/eqms/actop/releases/latest" {
			fmt.Fprint(w, `{"tag_name":"0.7.0"}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	mock := &cli.MockClient{VersionErr: errors.New("no such binary")}
	ck := &update.Checker{BaseURL: srv.URL, CacheDir: t.TempDir()}
	msg := checkUpdatesCmd(mock, config.Default(), "0.6.1", ck)().(updatesMsg)
	if msg.result.Count() != 1 || msg.result.Components[0].Name != "actop" {
		t.Errorf("want only actop reported, got %+v", msg.result.Components)
	}
}
```

Add the imports this needs to `ui/app_test.go`: `"errors"`, `"fmt"`, `"net/http"`, `"net/http/httptest"`, and `"github.com/eqms/actop/update"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/ -run TestUpdates -v; go test ./ui/ -run TestCheckUpdates -v`
Expected: FAIL — `undefined: updatesMsg`, `undefined: checkUpdatesCmd`.

- [ ] **Step 3: Write minimal implementation**

Append to `ui/messages.go` (and add `"github.com/eqms/actop/update"` to its imports):

```go
type updatesMsg struct{ result update.Result }
```

Append to `ui/commands.go` (imports gain `"github.com/eqms/actop/update"`):

```go
// updateCheckTimeout bounds the whole release check. It runs off the update
// loop, so a slow answer delays nothing visible.
const updateCheckTimeout = 10 * time.Second

// checkUpdatesCmd asks the CLI for the installed container version and the
// checker for the latest releases. It returns nil when the user opted out,
// so no HTTP client is constructed and the CLI is never touched.
func checkUpdatesCmd(c cli.Client, cfg config.Config, version string, ck *update.Checker) tea.Cmd {
	if !cfg.UpdateCheckEnabled() {
		return nil
	}
	if ck == nil {
		ck = &update.Checker{}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		local := map[string]string{"actop": version}
		// A missing or broken container CLI only drops that one component;
		// Run skips entries it cannot parse.
		if v, err := c.Version(ctx); err == nil {
			local["container"] = v
		}
		return updatesMsg{result: ck.Run(ctx, local)}
	}
}
```

In `ui/app.go`, add `"github.com/eqms/actop/update"` to the imports and two fields to `App`, next to `sysInfo`:

```go
	updates update.Result
	// checker is nil in production (the zero-value Checker is the real one);
	// tests inject one pointing at an httptest server.
	checker *update.Checker
```

Extend `Init`:

```go
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		fetchListCmd(a.client, a.allContainers),
		fetchStatsCmd(a.client),
		tickCmd(a.interval()),
		checkUpdatesCmd(a.client, a.cfg, a.version, a.checker),
	)
}
```

Add a case to `App.Update`, directly after the `systemStatusMsg` case:

```go
	case updatesMsg:
		a.updates = m.result
		return a, nil
```

In `ui/program_test.go`, add at the top of `TestProgramRendersAndQuits`, before the mock is built:

```go
	// The smoke test boots the real program, and Init fires the release
	// check. The suite must not depend on network access.
	t.Setenv(config.EnvNoUpdateCheck, "1")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -race`
Expected: PASS. Confirm the smoke test stays offline by running it with the network path in place: `go test ./ui/ -run TestProgramRendersAndQuits -race -v`.

- [ ] **Step 5: Commit**

```bash
git add ui/messages.go ui/commands.go ui/app.go ui/app_test.go ui/program_test.go
git commit -m "[ADD] ui: run the release check from Init and store its result"
```

---

### Task 7: Header badge

**Files:**
- Modify: `ui/app.go` (`header`, new `updateBadge`)
- Test: `ui/app_test.go` (append)

**Interfaces:**
- Consumes: `App.updates` (Task 6), existing `styleWarn`, `displayWidth`.
- Produces: `(*App) updateBadge() string`.

- [ ] **Step 1: Write the failing test**

Append to `ui/app_test.go`:

```go
func withUpdates(app *App, n int) {
	comps := make([]update.Component, 0, n)
	names := []string{"container", "actop"}
	for i := 0; i < n; i++ {
		comps = append(comps, update.Component{
			Name: names[i%len(names)], Current: "1.1.0", Latest: "1.2.0", Hint: "brew upgrade actop",
		})
	}
	app.Update(updatesMsg{result: update.Result{Components: comps}})
}

func TestHeaderShowsUpdateBadge(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	if strings.Contains(app.header(), "update") {
		t.Fatal("no result yet, but the header already advertises updates")
	}
	withUpdates(app, 2)
	if !strings.Contains(app.header(), "2 updates") {
		t.Errorf("header lacks the badge:\n%s", app.header())
	}
	app.updates = update.Result{}
	withUpdates(app, 1)
	if !strings.Contains(app.header(), "1 update") || strings.Contains(app.header(), "1 updates") {
		t.Errorf("singular badge wrong:\n%s", app.header())
	}
}

// The error banner owns the header while the API server is unreachable; an
// update hint must not compete with it.
func TestHeaderHidesBadgeWhileDisconnected(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	withUpdates(app, 2)
	app.connOK = false
	if strings.Contains(app.header(), "2 updates") {
		t.Errorf("badge must be suppressed while disconnected:\n%s", app.header())
	}
}

// On a narrow terminal the tab bar matters more than the hint.
func TestHeaderDropsBadgeWhenTooNarrow(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 40, 40
	withUpdates(app, 2)
	if strings.Contains(app.header(), "2 updates") {
		t.Errorf("badge must not push the tab bar out of a 40-column terminal:\n%s", app.header())
	}
}
```

Add `"strings"` to the `ui/app_test.go` imports if it is not there yet.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/ -run TestHeader -v`
Expected: FAIL — the header renders without a badge, so `TestHeaderShowsUpdateBadge` reports the missing "2 updates".

- [ ] **Step 3: Write minimal implementation**

Replace `App.header` in `ui/app.go` with this version (the plain-text copies exist so the width check measures glyphs, not ANSI escapes):

```go
func (a *App) header() string {
	leftPlain := fmt.Sprintf("actop v%s", a.version)
	left := styleHeader.Render(leftPlain)
	var tabs, tabsPlain strings.Builder
	tabs.WriteString("  ")
	tabsPlain.WriteString("  ")
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d]%s", i+1, name)
		if i == a.activeTab {
			tabs.WriteString(styleTitle.Render(label))
		} else {
			tabs.WriteString(styleDim.Render(label))
		}
		tabs.WriteString(" ")
		tabsPlain.WriteString(label + " ")
	}
	info := fmt.Sprintf(" %d containers · sort %s · poll %s",
		a.store.Len(), a.sortField, a.interval().Truncate(time.Millisecond))
	line := left + tabs.String() + styleDim.Render(info)

	if !a.connOK {
		banner := fmt.Sprintf("API server not reachable — retrying every %s (r = retry now)", a.interval())
		if a.sysDetail != "" {
			banner += " · " + safe(a.sysDetail)
		}
		stale := ""
		if !a.lastOK.IsZero() {
			stale = styleDim.Render(fmt.Sprintf("  showing data from %s", a.lastOK.Format("15:04:05")))
		}
		return line + "\n" + styleErr.Render(banner) + stale
	}

	// The badge is the least important thing on this line: it yields to the
	// error banner above and to a terminal too narrow for the tab bar.
	if badge := a.updateBadge(); badge != "" {
		plain := leftPlain + tabsPlain.String() + info
		if a.width == 0 || displayWidth(plain)+displayWidth(badge) <= a.width {
			line += styleWarn.Render(badge)
		}
	}

	if a.statusMsg != "" {
		// carries container IDs and CLI stderr; sanitized here rather than at
		// every assignment so no future call site can bypass it
		return line + "\n" + styleWarn.Render(safe(a.statusMsg))
	}
	return line
}

// updateBadge is the compact header hint; the system tab carries the detail.
func (a *App) updateBadge() string {
	switch n := a.updates.Count(); {
	case n == 0:
		return ""
	case n == 1:
		return "  ⬆ 1 update"
	default:
		return fmt.Sprintf("  ⬆ %d updates", n)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ui/ -race -v`
Expected: PASS, including the pre-existing header tests.

- [ ] **Step 5: Commit**

```bash
git add ui/app.go ui/app_test.go
git commit -m "[ADD] ui: update badge in the header"
```

---

### Task 8: `UPDATES` block in the system tab

**Files:**
- Modify: `ui/system.go`
- Test: `ui/app_test.go` (append)

**Interfaces:**
- Consumes: `App.updates` (Task 6), the local `line()` helper and `styleColHead` in `systemScreen.view`.
- Produces: rendering only; no new exported names.

- [ ] **Step 1: Write the failing test**

Append to `ui/app_test.go`:

```go
func TestSystemTabRendersUpdates(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	app.Update(updatesMsg{result: update.Result{Components: []update.Component{
		{Name: "container", Current: "1.1.0", Latest: "1.2.0", Hint: "/usr/local/bin/update-container.sh"},
		{Name: "actop", Current: "0.6.1", Latest: "0.7.0", Hint: "brew upgrade actop"},
	}}})
	view := newSystemScreen(app).view(200, 40)
	for _, want := range []string{
		"UPDATES",
		"container", "1.1.0 → 1.2.0 available", "/usr/local/bin/update-container.sh",
		"actop", "0.6.1 → 0.7.0 available", "brew upgrade actop",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("system tab is missing %q:\n%s", want, view)
		}
	}
}

// Nothing outdated means the section is absent entirely, not an empty header.
func TestSystemTabOmitsUpdatesWhenNone(t *testing.T) {
	app, _ := testApp()
	view := newSystemScreen(app).view(200, 40)
	if strings.Contains(view, "UPDATES") {
		t.Errorf("empty result must not render a section:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/ -run TestSystemTab -v`
Expected: FAIL — `TestSystemTabRendersUpdates` reports the missing "UPDATES".

- [ ] **Step 3: Write minimal implementation**

In `ui/system.go`, insert this block into `systemScreen.view` between the `RESOURCES` section (after the `volume size` line) and the `HOST` section:

```go
	// Names, versions and hints all originate in update/, which validates
	// remote tags against plain semver -- safe() is the second line of
	// defence, applied because everything rendered goes through it.
	if s.app.updates.Count() > 0 {
		b.WriteString("\n  " + styleColHead.Render("UPDATES") + "\n")
		for _, c := range s.app.updates.Components {
			line(safe(c.Name), styleWarn.Render(fmt.Sprintf("%s → %s available", safe(c.Current), safe(c.Latest))))
			line("", styleDim.Render(safe(c.Hint)))
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ui/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/system.go ui/app_test.go
git commit -m "[ADD] ui: UPDATES section in the system tab"
```

---

### Task 9: Lint, docs and release

**Files:**
- Modify: `VERSION`, `RELEASE_NOTES.md`, `README.md` (one bilingual file: a German half and an English half, each with its own `config.toml` example block)

**Interfaces:**
- Consumes: everything above.
- Produces: a releasable v0.7.0.

- [ ] **Step 1: Run the full quality gate**

Run: `make lint && make test && make build`
Expected: all three pass. If `gosec` flags the `os.ReadFile` in `update/cache.go`, the `#nosec G304` comment from Task 2 is already in place with its justification; do not add a blanket exclusion.

- [ ] **Step 2: Verify the opt-out actually opts out**

`make build` produces `./actop` in the repo root. Point the cache at a scratch directory and start the binary twice — once opted out, once not — quitting each time with `q`:

```bash
export XDG_CACHE_HOME=/tmp/actop-verify
rm -rf "$XDG_CACHE_HOME"
ACTOP_NO_UPDATE_CHECK=1 ./actop      # quit with q
ls "$XDG_CACHE_HOME/actop/" 2>&1     # expected: No such file or directory
./actop                              # quit with q
cat "$XDG_CACHE_HOME/actop/update.json"
```

Expected: the opted-out run leaves no cache directory behind; the second run writes `update.json` containing a `checkedAt` stamp and the fetched versions. This is the one manual step in the plan — it is the only way to confirm the real GitHub endpoint answers in the shape the tests assume.

- [ ] **Step 3: Bump the version**

Set the contents of `VERSION` to:

```
0.7.0
```

- [ ] **Step 4: Add release notes**

Add to the German half of `RELEASE_NOTES.md`, above the previous entry, matching the existing ISO-dated heading style:

```markdown
### v0.7.0 (2026-08-04)

- Update-Hinweis beim Start: actop prüft einmal täglich, ob eine neuere
  Version von Apple Container oder von actop vorliegt, und zeigt den
  passenden Befehl an. actop installiert nichts selbst.
- Der Hinweis erscheint als Badge in der Kopfzeile und ausführlich im
  System-Tab unter `UPDATES`.
- Abschaltbar mit `updateCheck = false` in `config.toml` oder mit
  `ACTOP_NO_UPDATE_CHECK=1` für einen einzelnen Start.
```

And the matching entry in the English half:

```markdown
### v0.7.0 (2026-08-04)

- Start-up update hint: actop checks once a day whether a newer Apple
  Container or actop release exists and shows the command that installs it.
  actop never installs anything itself.
- The hint appears as a badge in the header and in full under `UPDATES` in
  the system tab.
- Opt out with `updateCheck = false` in `config.toml`, or with
  `ACTOP_NO_UPDATE_CHECK=1` for a single run.
```

- [ ] **Step 5: Document the opt-out in the README**

`README.md` documents the config as a commented TOML block, once in the German half (around line 72) and once in the English half (around line 169). Add the key to **both** blocks, keeping the existing comment alignment:

German block:

```toml
updateCheck = true       # täglicher Versionscheck; ACTOP_NO_UPDATE_CHECK=1 deaktiviert ihn
```

English block:

```toml
updateCheck = true       # daily release check; ACTOP_NO_UPDATE_CHECK=1 turns it off
```

Both halves also have a `### Sicherheit` / security section. Add one bullet to each, since "actop talks to the network now" is exactly what a reader of that section wants to know:

```markdown
- Der Update-Check fragt einmal täglich `api.github.com` nach den neuesten
  Releases — ohne Token, ohne Telemetrie, und nur nach Versionsnummern.
  Abschaltbar per `updateCheck = false` oder `ACTOP_NO_UPDATE_CHECK=1`.
```

```markdown
- The update check queries `api.github.com` once a day for the latest
  releases — no token, no telemetry, version numbers only. Disable it with
  `updateCheck = false` or `ACTOP_NO_UPDATE_CHECK=1`.
```

- [ ] **Step 6: Commit**

```bash
git add VERSION RELEASE_NOTES.md README.md
git commit -m "[CHG] v0.7.0: release notes and version bump"
```

---

## Verification

After Task 9, the following must all hold:

- `make lint && make test && make build` pass.
- `go test ./... -race -count=1` passes with no network access (verify by running it with the network down, or trust that no test constructs a `Checker` without `BaseURL`).
- `grep -rn "api.github.com" --include=*_test.go .` returns nothing.
- No new direct dependency: `git diff <commit before Task 1> -- go.mod go.sum` is empty, and the `require` block still lists exactly five direct modules.
- Starting actop with `ACTOP_NO_UPDATE_CHECK=1` creates no `update.json`.
