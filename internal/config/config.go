// Package config loads TOML-based HUD configuration with defaults.
// LoadHud never returns nil and never returns an error — it fails open,
// using defaults whenever the config file is absent or unreadable.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/theme"
)

// Line represents a single rendered row in the statusline.
// Each widget name maps to a render function in the widget registry.
// Mode overrides the global style.mode for this specific line.
// Valid values: "" (inherit global), "plain", "powerline", "minimal".
type Line struct {
	Widgets []string `toml:"widgets"`
	Mode    string   `toml:"mode"`
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
		Display       string `toml:"display"`
		Value         string `toml:"value"`
		ShowBreakdown bool   `toml:"show_breakdown"`
	} `toml:"context"`
	Speed struct {
		WindowSecs int `toml:"window_secs"`
	} `toml:"speed"`
	Directory struct {
		Levels int    `toml:"levels"`
		Style  string `toml:"style"`
	} `toml:"directory"`
	Git struct {
		Dirty       bool `toml:"dirty"`
		AheadBehind bool `toml:"ahead_behind"`
		FileStats   bool `toml:"file_stats"`
	} `toml:"git"`
	Style struct {
		Separator  string `toml:"separator"`
		Icons      string `toml:"icons"`
		ColorLevel string `toml:"color_level"`
		Theme      string `toml:"theme"`
		// Mode controls the segment decoration style for all lines unless overridden per-line.
		// Accepted values: "plain" (default), "powerline", "minimal".
		Mode   string `toml:"mode"`
		Colors struct {
			Context  string `toml:"context"`
			Warning  string `toml:"warning"`
			Critical string `toml:"critical"`
		} `toml:"colors"`
	} `toml:"style"`
	Thresholds struct {
		// ContextWarning is the context usage percentage at which the widget
		// shifts to warning color. Default: 70.
		ContextWarning int `toml:"context_warning"`
		// ContextCritical is the context usage percentage at which the widget
		// shifts to critical color. Default: 85.
		ContextCritical int `toml:"context_critical"`
		// CostWarning is the session cost in USD at which the cost widget
		// shifts to warning color. Default: 5.00.
		CostWarning float64 `toml:"cost_warning"`
		// CostCritical is the session cost in USD at which the cost widget
		// shifts to critical color. Default: 10.00.
		CostCritical float64 `toml:"cost_critical"`
	} `toml:"thresholds"`

	// Theme holds the raw TOML overrides from [theme.overrides].
	// Each key is a widget name; the value has optional fg and bg fields.
	// After loading, ResolvedTheme is populated from Style.Theme + Theme.Overrides.
	Theme struct {
		Overrides map[string]theme.WidgetColors `toml:"overrides"`
	} `toml:"theme"`

	// Permission controls the permission-waiting widget.
	Permission struct {
		// ShowProject displays the project name of the waiting session next to the icon.
		// When false, only the icon is shown. Default: true.
		ShowProject bool `toml:"show_project"`
	} `toml:"permission"`

	// Usage controls the usage/rate-limit widget.
	Usage struct {
		// FiveHourThreshold is the minimum 5-hour usage percentage at which
		// the widget appears. Below this value the widget is hidden. Default: 0.
		FiveHourThreshold int `toml:"five_hour_threshold"`
		// SevenDayThreshold is the minimum 7-day usage percentage at which
		// the 7-day window is appended. Default: 80.
		SevenDayThreshold int `toml:"seven_day_threshold"`
		// CacheTTLSeconds overrides the success cache TTL. Default: 180.
		CacheTTLSeconds int `toml:"cache_ttl_seconds"`
	} `toml:"usage"`

	// Extra holds user-configured extra command settings. When Command is set,
	// the gather stage runs it and stores the result in RenderContext.ExtraOutput.
	Extra struct {
		Command string `toml:"command"`
	} `toml:"extra"`

	// ResolvedTheme is the effective per-widget color map after merging the
	// selected built-in theme with any custom [theme.overrides].
	// Populated by ResolveTheme() during LoadHud. Not read from TOML directly.
	ResolvedTheme theme.Theme `toml:"-"`
}

// DefaultLines returns the canonical default widget layout. It is defined
// here as the single source of truth and referenced by the "default" preset
// in the preset package. Returns a fresh slice each call so callers (including
// TOML decode) cannot mutate the canonical definition.
func DefaultLines() []Line {
	return []Line{
		{Widgets: []string{"model", "context", "project", "worktree", "todos", "duration", "permission"}},
		{Widgets: []string{"agents"}},
	}
}

// defaults returns a Config pre-populated with all default values.
func defaults() *Config {
	cfg := &Config{}

	cfg.Lines = DefaultLines()

	cfg.Model.ShowContextSize = true

	cfg.Context.BarWidth = 10
	cfg.Context.Display = "text"
	cfg.Context.Value = "percent"
	cfg.Context.ShowBreakdown = true

	cfg.Speed.WindowSecs = 30

	cfg.Directory.Levels = 1
	cfg.Directory.Style = "full"

	cfg.Git.Dirty = true
	cfg.Git.AheadBehind = true
	cfg.Git.FileStats = false

	cfg.Permission.ShowProject = true

	cfg.Usage.FiveHourThreshold = 0
	cfg.Usage.SevenDayThreshold = 80
	cfg.Usage.CacheTTLSeconds = 180

	cfg.Style.Separator = " | "
	cfg.Style.Icons = "nerdfont"
	cfg.Style.ColorLevel = "auto"
	cfg.Style.Theme = "default"
	cfg.Style.Mode = "plain"

	cfg.Style.Colors.Context = "green"
	cfg.Style.Colors.Warning = "yellow"
	cfg.Style.Colors.Critical = "red"

	cfg.Thresholds.ContextWarning = 70
	cfg.Thresholds.ContextCritical = 85
	cfg.Thresholds.CostWarning = 5.00
	cfg.Thresholds.CostCritical = 10.00

	return cfg
}

// ResolveTheme populates cfg.ResolvedTheme by loading the named built-in theme
// and merging any custom [theme.overrides] on top of it. Called after the TOML
// decode so that both built-in selection and user overrides are captured.
// Also used by the preset package to trigger a palette refresh after mutating
// Style fields.
func ResolveTheme(cfg *Config) {
	base := theme.Load(cfg.Style.Theme)
	if len(cfg.Theme.Overrides) > 0 {
		cfg.ResolvedTheme = theme.MergeOverrides(base, cfg.Theme.Overrides)
	} else {
		cfg.ResolvedTheme = base
	}
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
		ResolveTheme(cfg)
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ResolveTheme(cfg)
		return cfg
	}

	// Unmarshal on top of the defaults struct. Fields present in the TOML
	// file overwrite the defaults; absent fields keep their default values.
	// BurntSushi/toml does not zero out unmentioned struct fields, so this
	// overlay pattern is safe.
	if _, err := toml.Decode(string(data), cfg); err != nil {
		ResolveTheme(cfg)
		return cfg
	}

	ResolveTheme(cfg)
	return cfg
}
