package theme

import (
	"testing"
)

func TestLoad_knownTheme(t *testing.T) {
	for _, name := range BuiltinNames() {
		t.Run(name, func(t *testing.T) {
			th := Load(name)
			if th == nil {
				t.Fatalf("Load(%q) returned nil", name)
			}
			// Every built-in theme must have at least a model and context entry.
			if _, ok := th["model"]; !ok {
				t.Errorf("theme %q missing 'model' entry", name)
			}
			if _, ok := th["context"]; !ok {
				t.Errorf("theme %q missing 'context' entry", name)
			}
		})
	}
}

func TestLoad_unknownFallsBackToDefault(t *testing.T) {
	th := Load("nonexistent-theme")
	def := Load("default")

	if len(th) != len(def) {
		t.Errorf("fallback theme len=%d, want %d (default)", len(th), len(def))
	}

	// Spot-check a key entry.
	if th["model"] != def["model"] {
		t.Errorf("fallback model colors %+v, want %+v", th["model"], def["model"])
	}
}

func TestLoad_emptyNameFallsBackToDefault(t *testing.T) {
	th := Load("")
	def := Load("default")

	if th["context"] != def["context"] {
		t.Errorf("empty name: context colors %+v, want %+v", th["context"], def["context"])
	}
}

func TestMergeOverrides_appliesOverrides(t *testing.T) {
	base := Theme{
		"model":   {Fg: "#ffffff", Bg: "#000000"},
		"context": {Fg: "#aaaaaa", Bg: "#111111"},
		"git":     {Fg: "#bbbbbb", Bg: "#222222"},
	}

	overrides := map[string]WidgetColors{
		"model": {Fg: "#ff0000", Bg: "#0000ff"},
	}

	merged := MergeOverrides(base, overrides)

	if merged["model"].Fg != "#ff0000" {
		t.Errorf("override: model Fg = %q, want %q", merged["model"].Fg, "#ff0000")
	}
	if merged["model"].Bg != "#0000ff" {
		t.Errorf("override: model Bg = %q, want %q", merged["model"].Bg, "#0000ff")
	}

	// Non-overridden entries must be unchanged.
	if merged["context"] != base["context"] {
		t.Errorf("non-overridden context changed: got %+v, want %+v", merged["context"], base["context"])
	}
	if merged["git"] != base["git"] {
		t.Errorf("non-overridden git changed: got %+v, want %+v", merged["git"], base["git"])
	}
}

func TestMergeOverrides_addsNewWidgetEntry(t *testing.T) {
	base := Theme{
		"model": {Fg: "#ffffff", Bg: "#000000"},
	}
	overrides := map[string]WidgetColors{
		"custom-widget": {Fg: "#123456", Bg: "#abcdef"},
	}

	merged := MergeOverrides(base, overrides)
	if merged["custom-widget"].Fg != "#123456" {
		t.Errorf("new entry Fg = %q, want %q", merged["custom-widget"].Fg, "#123456")
	}
}

func TestMergeOverrides_doesNotMutateBase(t *testing.T) {
	base := Theme{
		"model": {Fg: "#ffffff", Bg: "#000000"},
	}
	overrides := map[string]WidgetColors{
		"model": {Fg: "#ff0000", Bg: "#0000ff"},
	}

	_ = MergeOverrides(base, overrides)

	// base must be unchanged after merge.
	if base["model"].Fg != "#ffffff" {
		t.Errorf("base mutated: model Fg = %q, want %q", base["model"].Fg, "#ffffff")
	}
}

func TestMergeOverrides_emptyOverrides(t *testing.T) {
	base := Theme{
		"model": {Fg: "#ffffff", Bg: "#000000"},
	}

	merged := MergeOverrides(base, nil)

	if merged["model"] != base["model"] {
		t.Errorf("empty overrides changed result: got %+v, want %+v", merged["model"], base["model"])
	}
}

func TestBuiltinNames_complete(t *testing.T) {
	names := BuiltinNames()
	if len(names) < 6 {
		t.Errorf("expected at least 6 built-in themes, got %d", len(names))
	}

	expected := []string{"default", "dark", "gruvbox", "nord", "rose-pine", "tokyo-night"}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range expected {
		if !nameSet[want] {
			t.Errorf("expected built-in theme %q not found in BuiltinNames()", want)
		}
	}
}

func TestAllBuiltinThemesHaveAllWidgets(t *testing.T) {
	widgets := []string{"model", "context", "directory", "git", "project", "env",
		"duration", "tools", "agents", "todos", "session", "thinking"}

	for _, name := range BuiltinNames() {
		th := Load(name)
		for _, w := range widgets {
			if _, ok := th[w]; !ok {
				t.Errorf("theme %q missing widget entry %q", name, w)
			}
		}
	}
}

// TestMergeOverrides_noOverrideUsesTheme verifies that a widget with no entry
// in overrides keeps the base theme's colors unchanged.
func TestMergeOverrides_noOverrideUsesTheme(t *testing.T) {
	base := Theme{
		"model":   {Fg: "#aabbcc", Bg: "#112233"},
		"context": {Fg: "#ddeeff", Bg: "#445566"},
	}

	// Override only context; model should keep its base values.
	overrides := map[string]WidgetColors{
		"context": {Fg: "#ff0000", Bg: "#0000ff"},
	}

	merged := MergeOverrides(base, overrides)

	if merged["model"].Fg != "#aabbcc" {
		t.Errorf("no-override model Fg: got %q, want %q", merged["model"].Fg, "#aabbcc")
	}
	if merged["model"].Bg != "#112233" {
		t.Errorf("no-override model Bg: got %q, want %q", merged["model"].Bg, "#112233")
	}
}

// TestMergeOverrides_fgOnlyOverride verifies that an override with only Fg set
// replaces the widget entry. Bg will be empty (zero value) because MergeOverrides
// replaces the entire entry rather than merging individual fields.
// Callers that want to preserve the theme's Bg while changing Fg should copy
// the base entry's Bg into the override before calling MergeOverrides.
func TestMergeOverrides_fgOnlyOverride(t *testing.T) {
	base := Theme{
		"model": {Fg: "#ffffff", Bg: "#123456"},
	}

	overrides := map[string]WidgetColors{
		"model": {Fg: "#ff8800"}, // Bg intentionally omitted
	}

	merged := MergeOverrides(base, overrides)

	if merged["model"].Fg != "#ff8800" {
		t.Errorf("fg-only override model Fg: got %q, want %q", merged["model"].Fg, "#ff8800")
	}
	// Bg is empty because the override entry replaced the entire base entry.
	if merged["model"].Bg != "" {
		t.Errorf("fg-only override model Bg: got %q, want %q (empty)", merged["model"].Bg, "")
	}
}

// TestMergeOverrides_bgOnlyOverride verifies that an override with only Bg set
// replaces the widget entry. Fg will be empty (zero value) for the same reason
// as the fg-only case: MergeOverrides replaces the full entry.
func TestMergeOverrides_bgOnlyOverride(t *testing.T) {
	base := Theme{
		"duration": {Fg: "244", Bg: ""},
	}

	overrides := map[string]WidgetColors{
		"duration": {Bg: "235"}, // Fg intentionally omitted
	}

	merged := MergeOverrides(base, overrides)

	// Fg is empty because the override entry replaced the entire base entry.
	if merged["duration"].Fg != "" {
		t.Errorf("bg-only override duration Fg: got %q, want %q (empty)", merged["duration"].Fg, "")
	}
	if merged["duration"].Bg != "235" {
		t.Errorf("bg-only override duration Bg: got %q, want %q", merged["duration"].Bg, "235")
	}
}

// TestMergeOverrides_namedAndIndexedColors verifies that named ANSI colors
// ("red", "cyan") and 256-color index strings ("42", "114") are accepted and
// stored as-is. MergeOverrides treats colors as opaque strings; interpretation
// is left to the renderer.
func TestMergeOverrides_namedAndIndexedColors(t *testing.T) {
	base := Theme{
		"git":   {Fg: "87", Bg: ""},
		"tools": {Fg: "75", Bg: ""},
		"model": {Fg: "#81a1c1", Bg: "#4c566a"},
	}

	overrides := map[string]WidgetColors{
		"git":   {Fg: "cyan", Bg: "black"},      // named ANSI colors
		"tools": {Fg: "42", Bg: "235"},          // 256-color index strings
		"model": {Fg: "#ff0000", Bg: "#001122"}, // hex
	}

	merged := MergeOverrides(base, overrides)

	if merged["git"].Fg != "cyan" {
		t.Errorf("git Fg: got %q, want %q", merged["git"].Fg, "cyan")
	}
	if merged["git"].Bg != "black" {
		t.Errorf("git Bg: got %q, want %q", merged["git"].Bg, "black")
	}
	if merged["tools"].Fg != "42" {
		t.Errorf("tools Fg: got %q, want %q", merged["tools"].Fg, "42")
	}
	if merged["tools"].Bg != "235" {
		t.Errorf("tools Bg: got %q, want %q", merged["tools"].Bg, "235")
	}
	if merged["model"].Fg != "#ff0000" {
		t.Errorf("model Fg: got %q, want %q", merged["model"].Fg, "#ff0000")
	}
}
