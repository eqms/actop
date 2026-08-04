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
