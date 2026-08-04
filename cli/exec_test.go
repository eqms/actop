package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBinary writes a shell script that stands in for the container CLI and
// returns a client wired to it.
func fakeBinary(t *testing.T, script string, limit int) *ExecClient {
	t.Helper()
	path := filepath.Join(t.TempDir(), "container")
	// #nosec G306 -- a stand-in binary has to be executable; it lives in a
	// per-test temp dir that only the test user can reach
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &ExecClient{binary: path, maxOutput: limit}
}

func TestRunReturnsOutput(t *testing.T) {
	c := fakeBinary(t, `printf '[{"id":"db"}]'`, 1024)
	items, err := c.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].ID != "db" {
		t.Errorf("unexpected items: %+v", items)
	}
}

// A container's labels, env and arguments are chosen by whoever built the
// image, so the JSON a poll returns must not be able to grow the process
// until the OS kills it.
func TestRunRejectsOversizedOutput(t *testing.T) {
	c := fakeBinary(t, `yes 0123456789abcdef`, 4096)
	start := time.Now()
	_, err := c.List(context.Background(), false)
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("err = %v, want ErrOutputTooLarge", err)
	}
	// Rejecting is not enough: the still-running producer must not hold the
	// caller. Without cmd.WaitDelay this returns only when the child dies.
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %s to abandon the oversized process, want prompt return", elapsed)
	}
}

// Output exactly at the cap is still valid: the limit must not be off by one.
func TestRunAcceptsOutputAtLimit(t *testing.T) {
	payload := `[{"id":"` + strings.Repeat("x", 100) + `"}]`
	c := fakeBinary(t, `printf '%s'`+" '"+payload+"'", len(payload))
	items, err := c.List(context.Background(), false)
	if err != nil {
		t.Fatalf("output of exactly the cap must be accepted, got %v", err)
	}
	if len(items) != 1 {
		t.Errorf("unexpected items: %+v", items)
	}
}

// Stderr is captured manually now that Output() is no longer used; without it
// the API-down and not-found classification silently stops working.
func TestRunClassifiesStderr(t *testing.T) {
	c := fakeBinary(t, `echo "XPC connection invalid" >&2; exit 1`, 1024)
	if _, err := c.List(context.Background(), false); !errors.Is(err, ErrAPIServerDown) {
		t.Errorf("err = %v, want ErrAPIServerDown", err)
	}

	c = fakeBinary(t, `echo "no such container" >&2; exit 1`, 1024)
	if _, err := c.Inspect(context.Background(), "gone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRunReportsExitCodeAndStderr(t *testing.T) {
	c := fakeBinary(t, `echo "boom" >&2; exit 3`, 1024)
	_, err := c.List(context.Background(), false)
	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CLIError", err)
	}
	if ce.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", ce.ExitCode)
	}
	if !strings.Contains(ce.Stderr, "boom") {
		t.Errorf("Stderr = %q, want it to contain \"boom\"", ce.Stderr)
	}
}

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

// A zero-value client must still work: New() is the documented constructor,
// but nothing in the type should depend on it having run.
func TestZeroValueClientHasDefaults(t *testing.T) {
	var c ExecClient
	if c.bin() != defaultBinary {
		t.Errorf("bin() = %q, want %q", c.bin(), defaultBinary)
	}
	if c.limit() != defaultMaxOutput {
		t.Errorf("limit() = %d, want %d", c.limit(), defaultMaxOutput)
	}
}
