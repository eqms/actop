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
