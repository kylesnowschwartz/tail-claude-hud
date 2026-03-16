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

	// Spec 1: three default lines with C+D hybrid layout
	if len(cfg.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(cfg.Lines))
	}
	assertWidgets(t, cfg.Lines[0].Widgets, []string{"thinking", "model", "context", "project", "todos", "duration"})
	assertWidgets(t, cfg.Lines[1].Widgets, []string{"agents"})
	assertWidgets(t, cfg.Lines[2].Widgets, []string{"tools"})

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
	if len(cfg.Lines) != 3 {
		t.Errorf("Lines: got %d, want default 3", len(cfg.Lines))
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

// TestDefaultLayoutIsThreeLinesHybrid verifies the layout:
// Line 1 = thinking+identity+health, Line 2 = agents (ephemeral), Line 3 = tools (ephemeral).
func TestDefaultLayoutIsThreeLinesHybrid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := LoadHud()
	if cfg == nil {
		t.Fatal("LoadHud returned nil")
	}

	if len(cfg.Lines) != 3 {
		t.Fatalf("default layout: want 3 lines, got %d", len(cfg.Lines))
	}

	assertWidgets(t, cfg.Lines[0].Widgets, []string{"thinking", "model", "context", "project", "todos", "duration"})
	assertWidgets(t, cfg.Lines[1].Widgets, []string{"agents"})
	assertWidgets(t, cfg.Lines[2].Widgets, []string{"tools"})
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
