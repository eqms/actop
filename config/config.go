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

// AllowedShells is the whitelist for the exec-shell feature. Only these
// values may be configured as `shell`; anything else is rejected.
var AllowedShells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"/bin/sh": true, "/bin/bash": true, "/bin/zsh": true,
	"/usr/bin/fish": true, "/usr/local/bin/fish": true,
}

// ShellAllowed reports whether s may be used as exec shell.
func ShellAllowed(s string) bool { return AllowedShells[s] }
