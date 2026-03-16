// Package config loads TOML-based HUD configuration with defaults.
// LoadHud never returns nil and never returns an error — it fails open,
// using defaults whenever the config file is absent or unreadable.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Line represents a single rendered row in the statusline.
// Each widget name maps to a render function in the widget registry.
type Line struct {
	Widgets []string `toml:"widgets"`
}

// Config holds all HUD settings. Defaults are applied first; the TOML file
// overlays only the fields it explicitly sets.
type Config struct {
	Lines []Line `toml:"line"`
	Model struct {
		ShowContextSize bool `toml:"show_context_size"`
	} `toml:"model"`
	Context struct {
		BarWidth      int    `toml:"bar_width"`
		Value         string `toml:"value"`
		ShowBreakdown bool   `toml:"show_breakdown"`
	} `toml:"context"`
	Directory struct {
		Levels int `toml:"levels"`
	} `toml:"directory"`
	Git struct {
		Dirty       bool `toml:"dirty"`
		AheadBehind bool `toml:"ahead_behind"`
		FileStats   bool `toml:"file_stats"`
	} `toml:"git"`
	Style struct {
		Separator string `toml:"separator"`
		Icons     string `toml:"icons"`
		Colors    struct {
			Context  string `toml:"context"`
			Warning  string `toml:"warning"`
			Critical string `toml:"critical"`
		} `toml:"colors"`
	} `toml:"style"`
}

// defaults returns a Config pre-populated with all default values.
func defaults() *Config {
	cfg := &Config{}

	cfg.Lines = []Line{
		{Widgets: []string{"thinking", "model", "context", "project", "todos", "duration"}},
		{Widgets: []string{"agents"}},
		{Widgets: []string{"tools"}},
	}

	cfg.Model.ShowContextSize = true

	cfg.Context.BarWidth = 10
	cfg.Context.Value = "percent"
	cfg.Context.ShowBreakdown = true

	cfg.Directory.Levels = 1

	cfg.Git.Dirty = true
	cfg.Git.AheadBehind = true
	cfg.Git.FileStats = false

	cfg.Style.Separator = " | "
	cfg.Style.Icons = "nerdfont"

	cfg.Style.Colors.Context = "green"
	cfg.Style.Colors.Warning = "yellow"
	cfg.Style.Colors.Critical = "red"

	return cfg
}

// configPath returns the first config file path that exists, checking XDG
// location first, then the legacy ~/.claude/plugins path.
// Returns empty string when neither path exists.
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(home, ".config", "tail-claude-hud", "config.toml"),
		filepath.Join(home, ".claude", "plugins", "tail-claude-hud", "config.toml"),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// LoadHud returns a Config with defaults overlaid by the TOML file at the
// XDG config location (or the legacy plugin path). It never returns nil and
// never returns an error — any failure to read or parse the config file
// results in the default config being returned.
func LoadHud() *Config {
	cfg := defaults()

	path := configPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	// Unmarshal on top of the defaults struct. Fields present in the TOML
	// file overwrite the defaults; absent fields keep their default values.
	// BurntSushi/toml does not zero out unmentioned struct fields, so this
	// overlay pattern is safe.
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return cfg
	}

	return cfg
}
