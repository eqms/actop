// Package config loads actop settings from a TOML file at
// $XDG_CONFIG_HOME/actop/config.toml (default ~/.config/actop/config.toml).
// A missing file yields defaults; a broken file is an error.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PollIntervalMs int      `toml:"pollIntervalMs"`
	SortField      string   `toml:"sortField"`
	SortReversed   bool     `toml:"sortReversed"`
	AllContainers  bool     `toml:"allContainers"`
	Shell          string   `toml:"shell"`
	Columns        []string `toml:"columns"`
	// UpdateCheck is a pointer so an explicit `updateCheck = false` stays
	// distinguishable from an absent key, which must default to true.
	UpdateCheck *bool `toml:"updateCheck"`
}

func Default() Config {
	return Config{
		PollIntervalMs: 2000,
		SortField:      "name",
		AllContainers:  true,
	}
}

// PollInterval returns the poll interval, floored at 500ms.
func (c Config) PollInterval() time.Duration {
	if c.PollIntervalMs < 500 {
		return 500 * time.Millisecond
	}
	return time.Duration(c.PollIntervalMs) * time.Millisecond
}

// Path returns the config file location.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "actop", "config.toml")
}

// Load reads the config file, returning defaults if it does not exist.
func Load() (Config, error) {
	cfg := Default()
	path := Path()
	if path == "" {
		return cfg, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to Path(), as ctop's S key does, and returns the path.
// The file is replaced atomically (temp file + rename) so a failed write
// never leaves a half-written config; comments in the old file are lost.
// A nil UpdateCheck stays absent rather than becoming false.
func Save(cfg Config) (string, error) {
	path := Path()
	if path == "" {
		return "", fmt.Errorf("no config location: neither XDG_CONFIG_HOME nor a home directory is set")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return path, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after the rename
	if err := toml.NewEncoder(tmp).Encode(cfg); err != nil {
		_ = tmp.Close()
		return path, fmt.Errorf("encode config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return path, err
	}
	// CreateTemp already uses 0600; the rename keeps it.
	return path, os.Rename(tmp.Name(), path)
}

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

// AllowedShells is the whitelist for the exec-shell feature. Only these
// values may be configured as `shell`; anything else is rejected.
var AllowedShells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"/bin/sh": true, "/bin/bash": true, "/bin/zsh": true,
	"/usr/bin/fish": true, "/usr/local/bin/fish": true,
}

// ShellAllowed reports whether s may be used as exec shell.
func ShellAllowed(s string) bool { return AllowedShells[s] }
