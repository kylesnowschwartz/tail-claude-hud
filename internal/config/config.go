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
		LineStats   bool `toml:"line_stats"`
	} `toml:"git"`
	Skills struct {
		// MaxAgeMins hides skill invocations older than this many minutes.
		// 0 keeps the full session log visible.
		MaxAgeMins int `toml:"max_age_mins"`
	} `toml:"skills"`
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
		// Display selects "text" (icon/percent + reset countdown, default) or
		// "bar" (block-character bar, labeled "5h"/"7d" since a bar alone can't
		// distinguish windows).
		Display string `toml:"display"`
		// BarWidth is the bar's cell count when Display is "bar". Default: 8.
		BarWidth int `toml:"bar_width"`
	} `toml:"usage"`

	// Worktree controls the worktree widget.
	Worktree struct {
		// ShowBranch appends the worktree's branch name after the name
		// (e.g. "wt:fix-bug feat/cache"). Default: true.
		ShowBranch bool `toml:"show_branch"`
	} `toml:"worktree"`

	// Notify controls opt-in attention signals emitted by the hook handler via
	// terminal escape sequences. Both default to false — the tool stays silent
	// unless explicitly enabled, since notifications can be noisy.
	Notify struct {
		// PermissionBell emits a desktop notification and terminal bell when
		// this session blocks waiting for permission approval. Default: false.
		PermissionBell bool `toml:"permission_bell"`
		// SessionTitle sets the terminal session title (project · branch) on
		// session start and resume. Default: false.
		SessionTitle bool `toml:"session_title"`
	} `toml:"notify"`

	// Extra holds user-configured extra command settings. When Command is set,
	// the gather stage runs it and stores the result in RenderContext.ExtraOutput.
	Extra struct {
		Command string `toml:"command"`
	} `toml:"extra"`

	// ResolvedTheme is the effective per-widget color map after merging the
	// selected built-in theme, custom [theme.overrides], and the active theme file.
	// Populated by ResolveTheme() during LoadHud. Not read from TOML directly.
	ResolvedTheme theme.Theme `toml:"-"`

	activeThemePath string
}

// DefaultLines returns the canonical default widget layout. It is defined
// here as the single source of truth and referenced by the "default" preset
// in the preset package. Returns a fresh slice each call so callers (including
// TOML decode) cannot mutate the canonical definition.
func DefaultLines() []Line {
	return []Line{
		{Widgets: []string{"model", "context", "project", "worktree", "todos", "duration", "permission"}},
		{Widgets: []string{"agents"}},
		{Widgets: []string{"tools"}},
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
	cfg.Git.LineStats = false

	cfg.Skills.MaxAgeMins = 0

	cfg.Permission.ShowProject = true

	cfg.Worktree.ShowBranch = true

	cfg.Usage.FiveHourThreshold = 0
	cfg.Usage.SevenDayThreshold = 80
	cfg.Usage.CacheTTLSeconds = 180
	cfg.Usage.Display = "text"
	cfg.Usage.BarWidth = 8

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

// ResolveTheme populates cfg.ResolvedTheme by loading the named built-in theme,
// merging any custom [theme.overrides], then layering theme-active.toml on top.
// Called after the TOML decode so that built-in selection and overrides are captured.
// Also used by the preset package to trigger a palette refresh after mutating
// Style fields.
func ResolveTheme(cfg *Config) {
	resolved := theme.Load(cfg.Style.Theme)
	if len(cfg.Theme.Overrides) > 0 {
		resolved = theme.MergeOverrides(resolved, cfg.Theme.Overrides)
	}

	if overrides := activeThemeOverrides(cfg.activeThemePath); len(overrides) > 0 {
		resolved = theme.MergeOverrides(resolved, overrides)
	}
	cfg.ResolvedTheme = resolved
}

// activeThemeOverrides loads the external theme map. All read and decode errors
// are ignored deliberately: statusline rendering must fall back to the resolved
// built-in theme and config overrides when theme-active.toml is unusable.
func activeThemeOverrides(path string) map[string]theme.WidgetColors {
	if path == "" {
		return nil
	}

	var overrides map[string]theme.WidgetColors
	if _, err := toml.DecodeFile(path, &overrides); err != nil {
		return nil
	}
	return overrides
}

// configPath returns the first config file path that exists, checking configDir
// first, then legacyConfigDir. Returns empty string when neither path exists.
func configPath(configDir, legacyConfigDir string) string {
	for _, dir := range []string{configDir, legacyConfigDir} {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, "config.toml")
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
	home, err := os.UserHomeDir()
	if err != nil {
		return loadHud("", "")
	}

	return loadHud(
		filepath.Join(home, ".config", "tail-claude-hud"),
		filepath.Join(home, ".claude", "plugins", "tail-claude-hud"),
	)
}

// loadHud is the testable core of LoadHud. Config directories are supplied by
// the caller so theme resolution does not depend on the process home directory.
func loadHud(configDir, legacyConfigDir string) *Config {
	cfg := defaults()
	if configDir != "" {
		cfg.activeThemePath = filepath.Join(configDir, "theme-active.toml")
	}

	path := configPath(configDir, legacyConfigDir)
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
