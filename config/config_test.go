package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.PollInterval() != 2*time.Second {
		t.Errorf("PollInterval = %s, want 2s", c.PollInterval())
	}
	if c.SortField != "name" || !c.AllContainers {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

// A too-small interval would turn the poll loop into a busy loop against the
// container CLI, so the floor is enforced regardless of what the file says.
func TestPollIntervalFloor(t *testing.T) {
	for _, ms := range []int{0, -1, 1, 499} {
		c := Config{PollIntervalMs: ms}
		if got := c.PollInterval(); got != 500*time.Millisecond {
			t.Errorf("PollIntervalMs=%d yields %s, want 500ms floor", ms, got)
		}
	}
	if got := (Config{PollIntervalMs: 500}).PollInterval(); got != 500*time.Millisecond {
		t.Errorf("500ms must pass through, got %s", got)
	}
	if got := (Config{PollIntervalMs: 3000}).PollInterval(); got != 3*time.Second {
		t.Errorf("PollInterval = %s, want 3s", got)
	}
}

func TestPathUsesXDGThenHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, want := Path(), filepath.Join("/xdg", "actop", "config.toml"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/picard")
	want := filepath.Join("/home/picard", ".config", "actop", "config.toml")
	if got := Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // exists, but holds no config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("missing file must not be an error, got %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("cfg = %+v, want defaults", cfg)
	}
}

func writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "actop")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsFile(t *testing.T) {
	writeConfig(t, `
pollIntervalMs = 5000
sortField = "cpu"
sortReversed = true
allContainers = false
shell = "/bin/zsh"
columns = ["name", "cpu"]
`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PollInterval() != 5*time.Second {
		t.Errorf("PollInterval = %s, want 5s", cfg.PollInterval())
	}
	if cfg.SortField != "cpu" || !cfg.SortReversed || cfg.AllContainers {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.Shell != "/bin/zsh" {
		t.Errorf("Shell = %q", cfg.Shell)
	}
	if len(cfg.Columns) != 2 || cfg.Columns[0] != "name" || cfg.Columns[1] != "cpu" {
		t.Errorf("Columns = %v", cfg.Columns)
	}
}

// A broken file is reported instead of being silently ignored, and the
// returned config stays usable so the caller can carry on with defaults.
func TestLoadBrokenFileErrorsWithDefaults(t *testing.T) {
	writeConfig(t, "pollIntervalMs = = 5")
	cfg, err := Load()
	if err == nil {
		t.Fatal("broken TOML must return an error")
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("cfg = %+v, want defaults alongside the error", cfg)
	}
}

// ShellAllowed is the only validation between the config file and exec, so
// anything outside the list -- especially values carrying shell syntax --
// must be rejected.
func TestShellAllowlist(t *testing.T) {
	for _, s := range []string{"sh", "bash", "zsh", "fish", "/bin/sh", "/bin/bash", "/bin/zsh", "/usr/bin/fish", "/usr/local/bin/fish"} {
		if !ShellAllowed(s) {
			t.Errorf("ShellAllowed(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"",
		"SH",
		"bash -c id",
		"sh;id",
		"sh id",
		"/bin/sh ",
		" /bin/sh",
		"/usr/bin/env sh",
		"../../bin/sh",
		"/tmp/evil",
		"$(id)",
		"`id`",
		"sh\nid",
		"python3",
	} {
		if ShellAllowed(s) {
			t.Errorf("ShellAllowed(%q) = true, want false", s)
		}
	}
}

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
