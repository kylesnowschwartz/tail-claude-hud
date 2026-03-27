package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig creates a config.toml file at the given directory path and
// returns the full file path.
func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

// TestDefaultsWhenNoFile verifies that LoadHud returns a fully-populated
// config with all defaults when no config file exists on disk.
func TestDefaultsWhenNoFile(t *testing.T) {
	// Point HOME at a temp dir that has no config file.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	// Default layout from config.DefaultLines.
	if len(cfg.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(cfg.Lines))
	}
	assertWidgets(t, cfg.Lines[0].Widgets, []string{"model", "context", "project", "worktree", "todos", "duration", "permission"})
	assertWidgets(t, cfg.Lines[1].Widgets, []string{"agents"})

	// Spec 4: default Icons
	if cfg.Style.Icons != "nerdfont" {
		t.Errorf("Style.Icons: got %q, want %q", cfg.Style.Icons, "nerdfont")
	}

	// Spec 5: default Separator
	if cfg.Style.Separator != " | " {
		t.Errorf("Style.Separator: got %q, want %q", cfg.Style.Separator, " | ")
	}

	// Model defaults
	if !cfg.Model.ShowContextSize {
		t.Error("Model.ShowContextSize: want true")
	}

	// Context defaults
	if cfg.Context.BarWidth != 10 {
		t.Errorf("Context.BarWidth: got %d, want 10", cfg.Context.BarWidth)
	}
	if cfg.Context.Value != "percent" {
		t.Errorf("Context.Value: got %q, want %q", cfg.Context.Value, "percent")
	}
	if !cfg.Context.ShowBreakdown {
		t.Error("Context.ShowBreakdown: want true")
	}

	// Directory defaults
	if cfg.Directory.Levels != 1 {
		t.Errorf("Directory.Levels: got %d, want 1", cfg.Directory.Levels)
	}

	// Git defaults
	if !cfg.Git.Dirty {
		t.Error("Git.Dirty: want true")
	}
	if !cfg.Git.AheadBehind {
		t.Error("Git.AheadBehind: want true (project widget needs ahead/behind data)")
	}
	if cfg.Git.FileStats {
		t.Error("Git.FileStats: want false")
	}

	// Color defaults
	if cfg.Style.Colors.Context != "green" {
		t.Errorf("Colors.Context: got %q, want %q", cfg.Style.Colors.Context, "green")
	}
	if cfg.Style.Colors.Warning != "yellow" {
		t.Errorf("Colors.Warning: got %q, want %q", cfg.Style.Colors.Warning, "yellow")
	}
	if cfg.Style.Colors.Critical != "red" {
		t.Errorf("Colors.Critical: got %q, want %q", cfg.Style.Colors.Critical, "red")
	}
}

// TestLoadFromXDGPath verifies that LoadHud reads a config.toml from
// ~/.config/tail-claude-hud/ and overlays it on defaults (spec 2).
func TestLoadFromXDGPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[[line]]
widgets = ["model", "git"]

[[line]]
widgets = ["tools", "agents"]

[style]
separator = " > "
icons = "unicode"
`)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	// Lines from file
	if len(cfg.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(cfg.Lines))
	}
	assertWidgets(t, cfg.Lines[0].Widgets, []string{"model", "git"})
	assertWidgets(t, cfg.Lines[1].Widgets, []string{"tools", "agents"})

	if cfg.Style.Separator != " > " {
		t.Errorf("Separator: got %q, want %q", cfg.Style.Separator, " > ")
	}
	if cfg.Style.Icons != "unicode" {
		t.Errorf("Icons: got %q, want %q", cfg.Style.Icons, "unicode")
	}
}

// TestLegacyPluginPath verifies that LoadHud falls back to the
// ~/.claude/plugins/tail-claude-hud/ path when the XDG dir is absent.
func TestLegacyPluginPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".claude", "plugins", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[style]
separator = " :: "
`)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	if cfg.Style.Separator != " :: " {
		t.Errorf("Separator: got %q, want %q", cfg.Style.Separator, " :: ")
	}

	// Defaults preserved for fields not in the file (spec 3).
	if cfg.Style.Icons != "nerdfont" {
		t.Errorf("Icons should keep default, got %q", cfg.Style.Icons)
	}
}

// TestXDGPreferredOverLegacy verifies that when both paths exist, the XDG
// location takes precedence over the legacy plugin path.
func TestXDGPreferredOverLegacy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	xdgDir := filepath.Join(tmp, ".config", "tail-claude-hud")
	legacyDir := filepath.Join(tmp, ".claude", "plugins", "tail-claude-hud")

	for _, dir := range []string{xdgDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeConfig(t, xdgDir, `[style]
separator = " XDG "
`)
	writeConfig(t, legacyDir, `[style]
separator = " LEGACY "
`)

	cfg := LoadHud()
	if cfg.Style.Separator != " XDG " {
		t.Errorf("expected XDG config to win, got separator %q", cfg.Style.Separator)
	}
}

// TestPartialOverlayPreservesDefaults verifies spec 3: a TOML file that sets
// only some fields leaves the remaining fields at their default values.
func TestPartialOverlayPreservesDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Only override bar_width; all other context fields should keep defaults.
	writeConfig(t, dir, `
[context]
bar_width = 20
`)

	cfg := LoadHud()

	if cfg.Context.BarWidth != 20 {
		t.Errorf("BarWidth: got %d, want 20", cfg.Context.BarWidth)
	}
	// Defaults preserved:
	if cfg.Context.Value != "percent" {
		t.Errorf("Value should keep default %q, got %q", "percent", cfg.Context.Value)
	}
	if !cfg.Context.ShowBreakdown {
		t.Error("ShowBreakdown should keep default true")
	}
	// Unrelated section also keeps defaults:
	if cfg.Style.Icons != "nerdfont" {
		t.Errorf("Icons should keep default, got %q", cfg.Style.Icons)
	}
	if cfg.Style.Separator != " | " {
		t.Errorf("Separator should keep default, got %q", cfg.Style.Separator)
	}
	if cfg.Directory.Levels != 1 {
		t.Errorf("Directory.Levels should keep default, got %d", cfg.Directory.Levels)
	}
}

// TestInvalidTOMLFallsBackToDefaults verifies that a malformed config file
// causes LoadHud to return defaults rather than a partial/zero-value config.
func TestInvalidTOMLFallsBackToDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `this is not valid toml ][[[`)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil on parse error")
	}

	// Should have received unmodified defaults.
	if cfg.Style.Icons != "nerdfont" {
		t.Errorf("Icons: got %q, want default %q", cfg.Style.Icons, "nerdfont")
	}
	if len(cfg.Lines) != 2 {
		t.Errorf("Lines: got %d, want default 2", len(cfg.Lines))
	}
}

// TestDefaultsNeverReturnsNil is a paranoia check that LoadHud never returns nil.
func TestDefaultsNeverReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	// HOME with no config dir at all.
	t.Setenv("HOME", tmp)

	if cfg := LoadHud(); cfg == nil {
		t.Error("LoadHud returned nil")
	}
}

// TestDefaultLayoutIsTwoLines verifies the layout from config.DefaultLines:
// Line 1 = identity+health+worktree, Line 2 = agents (ephemeral).
func TestDefaultLayoutIsTwoLines(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	if len(cfg.Lines) != 2 {
		t.Fatalf("default layout: want 2 lines, got %d", len(cfg.Lines))
	}

	assertWidgets(t, cfg.Lines[0].Widgets, []string{"model", "context", "project", "worktree", "todos", "duration", "permission"})
	assertWidgets(t, cfg.Lines[1].Widgets, []string{"agents"})
}

// TestDefaultEnvWidgetAbsent verifies that "env" is not present in the default layout
// (it remains available as an opt-in widget but is not shown by default).
func TestDefaultEnvWidgetAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	for i, line := range cfg.Lines {
		for _, w := range line.Widgets {
			if w == "env" {
				t.Errorf("default layout line %d contains 'env' widget; it should be opt-in only", i+1)
			}
		}
	}
}

// TestDefaultThemeIsResolved verifies that LoadHud populates ResolvedTheme
// from the default theme when no config file is present.
func TestDefaultThemeIsResolved(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := LoadHud()
	if cfg.ResolvedTheme == nil {
		t.Fatal("ResolvedTheme is nil after LoadHud with no config file")
	}
	// Default theme should include entries for all standard widgets.
	for _, w := range []string{"model", "context", "git", "tools", "agents"} {
		if _, ok := cfg.ResolvedTheme[w]; !ok {
			t.Errorf("ResolvedTheme missing entry for widget %q", w)
		}
	}
}

// TestThemeSelectionViaConfig verifies that style.theme = "nord" loads the
// nord built-in theme into ResolvedTheme.
func TestThemeSelectionViaConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[style]
theme = "nord"
`)

	cfg := LoadHud()

	if cfg.Style.Theme != "nord" {
		t.Errorf("Style.Theme: got %q, want %q", cfg.Style.Theme, "nord")
	}

	// Nord model entry should differ from default.
	if cfg.ResolvedTheme["model"].Bg == "" {
		t.Error("nord theme: model Bg should be non-empty")
	}
}

// TestThemeCustomOverridesMerge verifies that [theme.overrides] entries
// override the selected built-in theme while non-overridden entries remain.
func TestThemeCustomOverridesMerge(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[style]
theme = "nord"

[theme.overrides.model]
fg = "#ff0000"
bg = "#0000ff"
`)

	cfg := LoadHud()

	// The override should replace nord's model colors.
	if cfg.ResolvedTheme["model"].Fg != "#ff0000" {
		t.Errorf("override model Fg: got %q, want %q", cfg.ResolvedTheme["model"].Fg, "#ff0000")
	}
	if cfg.ResolvedTheme["model"].Bg != "#0000ff" {
		t.Errorf("override model Bg: got %q, want %q", cfg.ResolvedTheme["model"].Bg, "#0000ff")
	}

	// Other entries from nord must be present and unchanged.
	nordGitBg := "#3b4252"
	if cfg.ResolvedTheme["git"].Bg != nordGitBg {
		t.Errorf("non-overridden git Bg: got %q, want %q", cfg.ResolvedTheme["git"].Bg, nordGitBg)
	}
}

// TestUnknownThemeFallsBackToDefault verifies that an unrecognized theme name
// causes ResolvedTheme to contain the default theme's colors.
func TestUnknownThemeFallsBackToDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[style]
theme = "nonexistent-theme-xyz"
`)

	cfg := LoadHud()

	if cfg.ResolvedTheme == nil {
		t.Fatal("ResolvedTheme is nil for unknown theme")
	}
	// Should still have model entry (from default fallback).
	if _, ok := cfg.ResolvedTheme["model"]; !ok {
		t.Error("fallback ResolvedTheme missing 'model' entry")
	}
}

// TestThemeOverride_noOverrideUsesThemeDefaults verifies that a widget with no
// entry in [theme.overrides] keeps the built-in theme's colors in ResolvedTheme.
func TestThemeOverride_noOverrideUsesThemeDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Override only the model widget; duration should keep the default theme's value.
	writeConfig(t, dir, `
[style]
theme = "default"

[theme.overrides.model]
fg = "#ff0000"
bg = "#001122"
`)

	cfg := LoadHud()

	// duration is not overridden; it must equal the default theme's entry.
	defaultDurationFg := "244" // from theme.defaultTheme
	if cfg.ResolvedTheme["duration"].Fg != defaultDurationFg {
		t.Errorf("duration (no override) Fg: got %q, want %q", cfg.ResolvedTheme["duration"].Fg, defaultDurationFg)
	}
}

// TestThemeOverride_fgOnly verifies that a [theme.overrides] entry with only fg
// set updates ResolvedTheme for that widget. Bg will be empty because
// MergeOverrides replaces the entire widget entry.
func TestThemeOverride_fgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[theme.overrides.model]
fg = "#ff8800"
`)

	cfg := LoadHud()

	if cfg.ResolvedTheme["model"].Fg != "#ff8800" {
		t.Errorf("fg-only override model Fg: got %q, want %q", cfg.ResolvedTheme["model"].Fg, "#ff8800")
	}
	// Bg is zeroed out by the replace-entire-entry semantics.
	if cfg.ResolvedTheme["model"].Bg != "" {
		t.Errorf("fg-only override model Bg: got %q, want empty", cfg.ResolvedTheme["model"].Bg)
	}
}

// TestThemeOverride_bgOnly verifies that a [theme.overrides] entry with only bg
// set updates ResolvedTheme for that widget. Fg will be empty.
func TestThemeOverride_bgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[theme.overrides.duration]
bg = "235"
`)

	cfg := LoadHud()

	if cfg.ResolvedTheme["duration"].Fg != "" {
		t.Errorf("bg-only override duration Fg: got %q, want empty", cfg.ResolvedTheme["duration"].Fg)
	}
	if cfg.ResolvedTheme["duration"].Bg != "235" {
		t.Errorf("bg-only override duration Bg: got %q, want %q", cfg.ResolvedTheme["duration"].Bg, "235")
	}
}

// TestThemeOverride_bothFgAndBg verifies that setting both fg and bg in
// [theme.overrides] correctly replaces both fields in ResolvedTheme.
func TestThemeOverride_bothFgAndBg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[theme.overrides.context]
fg = "cyan"
bg = "#1a1a2e"

[theme.overrides.git]
fg = "42"
bg = "235"
`)

	cfg := LoadHud()

	// context: named ANSI fg + hex bg
	if cfg.ResolvedTheme["context"].Fg != "cyan" {
		t.Errorf("context Fg: got %q, want %q", cfg.ResolvedTheme["context"].Fg, "cyan")
	}
	if cfg.ResolvedTheme["context"].Bg != "#1a1a2e" {
		t.Errorf("context Bg: got %q, want %q", cfg.ResolvedTheme["context"].Bg, "#1a1a2e")
	}

	// git: 256-color fg + 256-color bg
	if cfg.ResolvedTheme["git"].Fg != "42" {
		t.Errorf("git Fg: got %q, want %q", cfg.ResolvedTheme["git"].Fg, "42")
	}
	if cfg.ResolvedTheme["git"].Bg != "235" {
		t.Errorf("git Bg: got %q, want %q", cfg.ResolvedTheme["git"].Bg, "235")
	}
}

// TestThemeOverride_colorFormats verifies that all three supported color formats
// (hex #RRGGBB, 256-color index, named ANSI) round-trip through TOML and
// appear correctly in ResolvedTheme.
func TestThemeOverride_colorFormats(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeConfig(t, dir, `
[theme.overrides.model]
fg = "#ff8800"

[theme.overrides.project]
fg = "114"

[theme.overrides.git]
fg = "green"
`)

	cfg := LoadHud()

	cases := []struct {
		widget string
		wantFg string
	}{
		{"model", "#ff8800"}, // hex
		{"project", "114"},   // 256-color index
		{"git", "green"},     // named ANSI
	}
	for _, tc := range cases {
		got := cfg.ResolvedTheme[tc.widget].Fg
		if got != tc.wantFg {
			t.Errorf("widget %q Fg: got %q, want %q", tc.widget, got, tc.wantFg)
		}
	}
}

// assertWidgets fails the test if got and want differ in length or content.
func assertWidgets(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("widget list length: got %d %v, want %d %v", len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("widget[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
