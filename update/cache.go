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
