package preset_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/preset"
)

func TestLoadValidPreset(t *testing.T) {
	p, ok := preset.Load("default")
	if !ok {
		t.Fatal("Load(\"default\") returned false, want true")
	}
	if p.Name != "default" {
		t.Errorf("Name = %q, want %q", p.Name, "default")
	}
}

func TestLoadUnknownPreset(t *testing.T) {
	p, ok := preset.Load("nonexistent")
	if ok {
		t.Fatal("Load(\"nonexistent\") returned true, want false")
	}
	if p.Name != "" || p.Lines != nil {
		t.Errorf("Load(\"nonexistent\") returned non-zero Preset: %+v", p)
	}
}

func TestBuiltinNamesReturnsFiveSorted(t *testing.T) {
	names := preset.BuiltinNames()
	if len(names) != 5 {
		t.Errorf("BuiltinNames() returned %d names, want 5: %v", len(names), names)
	}
	for i := 1; i < len(names); i++ {
		if names[i] <= names[i-1] {
			t.Errorf("BuiltinNames() not sorted at index %d: %v", i, names)
		}
	}
}

func TestAllBuiltinsLoadSuccessfully(t *testing.T) {
	for _, name := range preset.BuiltinNames() {
		t.Run(name, func(t *testing.T) {
			p, ok := preset.Load(name)
			if !ok {
				t.Fatalf("Load(%q) returned false", name)
			}
			if p.Name == "" {
				t.Errorf("preset %q has empty Name", name)
			}
			if len(p.Lines) == 0 {
				t.Errorf("preset %q has no Lines", name)
			}
			for i, line := range p.Lines {
				if len(line.Widgets) == 0 {
					t.Errorf("preset %q line %d has no widgets", name, i)
				}
			}
		})
	}
}

// TestDefaultPresetMatchesConfigDefaults verifies the default preset
// references config.DefaultLines — the single source of truth for layout.
func TestDefaultPresetMatchesConfigDefaults(t *testing.T) {
	p, ok := preset.Load("default")
	if !ok {
		t.Fatal("Load(\"default\") returned false")
	}

	defaults := config.DefaultLines()
	if len(p.Lines) != len(defaults) {
		t.Fatalf("default preset has %d lines, want %d (config.DefaultLines)", len(p.Lines), len(defaults))
	}

	for i, want := range defaults {
		assertWidgets(t, fmt.Sprintf("line %d", i), p.Lines[i].Widgets, want.Widgets)
	}
}

// TestApplyPresetOverwritesLayout verifies spec 1: ApplyPreset replaces Lines.
func TestApplyPresetOverwritesLayout(t *testing.T) {
	cfg := buildDefaultConfig(t)

	p := preset.Preset{
		Name:  "compact",
		Lines: []config.Line{{Widgets: []string{"model", "context"}}},
	}

	preset.ApplyPreset(cfg, p)

	if len(cfg.Lines) != 1 {
		t.Fatalf("Lines: got %d, want 1", len(cfg.Lines))
	}
	assertWidgets(t, "line 0", cfg.Lines[0].Widgets, []string{"model", "context"})
}

// TestApplyPresetOverwritesStyleFields verifies spec 2: ApplyPreset sets
// Style.Separator, Icons, Mode, Theme, and Directory.Style.
func TestApplyPresetOverwritesStyleFields(t *testing.T) {
	cfg := buildDefaultConfig(t)

	p := preset.Preset{
		Name:           "powerline-test",
		Separator:      " > ",
		Icons:          "unicode",
		Mode:           "powerline",
		Theme:          "nord",
		DirectoryStyle: "basename",
	}

	preset.ApplyPreset(cfg, p)

	if cfg.Style.Separator != " > " {
		t.Errorf("Separator: got %q, want %q", cfg.Style.Separator, " > ")
	}
	if cfg.Style.Icons != "unicode" {
		t.Errorf("Icons: got %q, want %q", cfg.Style.Icons, "unicode")
	}
	if cfg.Style.Mode != "powerline" {
		t.Errorf("Mode: got %q, want %q", cfg.Style.Mode, "powerline")
	}
	if cfg.Style.Theme != "nord" {
		t.Errorf("Theme: got %q, want %q", cfg.Style.Theme, "nord")
	}
	if cfg.Directory.Style != "basename" {
		t.Errorf("Directory.Style: got %q, want %q", cfg.Directory.Style, "basename")
	}
}

// TestApplyPresetPreservesThresholds verifies spec 3: ApplyPreset does not
// touch Thresholds, Context, Git, Speed, or Theme.Overrides.
func TestApplyPresetPreservesThresholds(t *testing.T) {
	cfg := buildDefaultConfig(t)

	// Set non-default values on all protected fields.
	cfg.Thresholds.ContextWarning = 50
	cfg.Thresholds.ContextCritical = 80
	cfg.Thresholds.CostWarning = 2.50
	cfg.Thresholds.CostCritical = 7.50
	cfg.Context.BarWidth = 20
	cfg.Context.Value = "tokens"
	cfg.Context.ShowBreakdown = false
	cfg.Git.FileStats = true
	cfg.Speed.WindowSecs = 120

	p := preset.Preset{
		Name:      "full",
		Separator: " :: ",
		Icons:     "ascii",
		Mode:      "plain",
		Theme:     "default",
		Lines:     []config.Line{{Widgets: []string{"model"}}},
	}

	preset.ApplyPreset(cfg, p)

	// Protected fields must be unchanged.
	if cfg.Thresholds.ContextWarning != 50 {
		t.Errorf("Thresholds.ContextWarning: got %d, want 50", cfg.Thresholds.ContextWarning)
	}
	if cfg.Thresholds.ContextCritical != 80 {
		t.Errorf("Thresholds.ContextCritical: got %d, want 80", cfg.Thresholds.ContextCritical)
	}
	if cfg.Thresholds.CostWarning != 2.50 {
		t.Errorf("Thresholds.CostWarning: got %v, want 2.50", cfg.Thresholds.CostWarning)
	}
	if cfg.Thresholds.CostCritical != 7.50 {
		t.Errorf("Thresholds.CostCritical: got %v, want 7.50", cfg.Thresholds.CostCritical)
	}
	if cfg.Context.BarWidth != 20 {
		t.Errorf("Context.BarWidth: got %d, want 20", cfg.Context.BarWidth)
	}
	if cfg.Context.Value != "tokens" {
		t.Errorf("Context.Value: got %q, want %q", cfg.Context.Value, "tokens")
	}
	if cfg.Context.ShowBreakdown != false {
		t.Error("Context.ShowBreakdown: should still be false")
	}
	if !cfg.Git.FileStats {
		t.Error("Git.FileStats: should still be true")
	}
	if cfg.Speed.WindowSecs != 120 {
		t.Errorf("Speed.WindowSecs: got %d, want 120", cfg.Speed.WindowSecs)
	}
}

// TestApplyPresetResolvesTheme verifies spec 4: ApplyPreset calls resolveTheme,
// populating ResolvedTheme for the new theme name.
func TestApplyPresetResolvesTheme(t *testing.T) {
	cfg := buildDefaultConfig(t)

	// Before applying: ResolvedTheme should be populated for the default theme.
	if cfg.ResolvedTheme == nil {
		t.Fatal("ResolvedTheme is nil after LoadHud")
	}

	p := preset.Preset{
		Name:  "theme-test",
		Theme: "nord",
	}

	preset.ApplyPreset(cfg, p)

	// After applying: ResolvedTheme must be re-resolved for the nord theme.
	if cfg.ResolvedTheme == nil {
		t.Fatal("ResolvedTheme is nil after ApplyPreset")
	}
	// Nord theme defines per-widget colors; model should have a Bg set.
	if cfg.ResolvedTheme["model"].Bg == "" {
		t.Error("nord theme: model Bg should be non-empty after resolveTheme")
	}
}

// TestLoadHudWithPresetEmptyName verifies that an empty preset name returns
// the default config without modification.
func TestLoadHudWithPresetEmptyName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := preset.LoadHudWithPreset("")
	if cfg == nil {
		t.Fatal("LoadHudWithPreset returned nil")
	}
	if len(cfg.Lines) != 2 {
		t.Errorf("Lines: got %d, want 2 (default)", len(cfg.Lines))
	}
}

// TestLoadHudWithPresetUnknownName verifies that an unknown preset name falls
// back to the default config unchanged.
func TestLoadHudWithPresetUnknownName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := preset.LoadHudWithPreset("does-not-exist")
	if cfg == nil {
		t.Fatal("LoadHudWithPreset returned nil")
	}
	if len(cfg.Lines) != 2 {
		t.Errorf("Lines: got %d, want 2 (default)", len(cfg.Lines))
	}
}

// TestLoadHudWithPresetAppliesCompact verifies that the "compact" built-in
// preset is applied correctly by LoadHudWithPreset.
func TestLoadHudWithPresetAppliesCompact(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := preset.LoadHudWithPreset("compact")
	if cfg == nil {
		t.Fatal("LoadHudWithPreset returned nil")
	}

	p, _ := preset.Load("compact")
	if len(cfg.Lines) != len(p.Lines) {
		t.Errorf("Lines: got %d, want %d (compact preset)", len(cfg.Lines), len(p.Lines))
	}
	if cfg.Directory.Style != p.DirectoryStyle {
		t.Errorf("Directory.Style: got %q, want %q", cfg.Directory.Style, p.DirectoryStyle)
	}
}

// buildDefaultConfig is a test helper that returns a config.LoadHud() result
// with HOME pointed at a temp dir so no user config interferes.
func buildDefaultConfig(t *testing.T) *config.Config {
	t.Helper()
	tmp := t.TempDir()
	if err := os.Setenv("HOME", tmp); err != nil {
		t.Fatalf("setenv HOME: %v", err)
	}
	t.Cleanup(func() { os.Unsetenv("HOME") })
	return config.LoadHud()
}

func assertWidgets(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v (len %d), want %v (len %d)", label, got, len(got), want, len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s widget[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}
