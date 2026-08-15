package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/preset"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/theme"
)

// makeTestConfig builds a Config with the given lines and a dark theme
// resolved into ResolvedTheme, suitable for powerline rendering tests.
func makeTestConfig(lines []config.Line) *config.Config {
	cfg := config.LoadHud()
	cfg.Style.Theme = "dark"
	cfg.Lines = lines
	config.ResolveTheme(cfg)
	return cfg
}

// makeResults builds a slice of WidgetResult and corresponding names from
// (name, text) pairs, for direct testing of renderPowerline. Both Text and
// PlainText are set to the same value since powerline uses PlainText.
func makeResults(pairs ...string) ([]widget.WidgetResult, []string) {
	var results []widget.WidgetResult
	var names []string
	for i := 0; i+1 < len(pairs); i += 2 {
		names = append(names, pairs[i])
		results = append(results, widget.WidgetResult{
			Text:      pairs[i+1],
			PlainText: pairs[i+1],
		})
	}
	return results, names
}

func TestRenderPowerline_NonEmptyOutput(t *testing.T) {
	results, names := makeResults("model", "Sonnet", "context", "50%")
	cfg := makeTestConfig(nil)

	line := renderPowerline(results, names, cfg)
	if line == "" {
		t.Fatal("renderPowerline returned empty string for non-empty input")
	}
	stripped := ansi.Strip(line)
	if !strings.Contains(stripped, "Sonnet") {
		t.Errorf("expected 'Sonnet' in powerline output, got %q", stripped)
	}
}

func TestRenderPowerline_EmptyWhenAllResultsEmpty(t *testing.T) {
	results := []widget.WidgetResult{{Text: ""}, {Text: ""}}
	names := []string{"model", "context"}
	cfg := makeTestConfig(nil)

	line := renderPowerline(results, names, cfg)
	if line != "" {
		t.Errorf("expected empty output when all results are empty, got %q", line)
	}
}

func TestRenderPowerline_ContainsArrowGlyph(t *testing.T) {
	results, names := makeResults("model", "Opus", "context", "75%")
	cfg := makeTestConfig(nil)

	line := renderPowerline(results, names, cfg)
	if !strings.Contains(line, powerlineArrow) {
		t.Errorf("expected powerline arrow %q in output, got %q", powerlineArrow, line)
	}
}

func TestRenderPowerline_ContainsStartCap(t *testing.T) {
	results, names := makeResults("model", "Haiku", "context", "20%")
	cfg := makeTestConfig(nil)

	line := renderPowerline(results, names, cfg)
	if !strings.Contains(line, powerlineStartCap) {
		t.Errorf("expected start cap %q in output, got %q", powerlineStartCap, line)
	}
}

func TestRenderPowerline_DistinctBgColors(t *testing.T) {
	// model bg="#2d2d2d", context bg="#4a5568" in dark theme — both must appear
	// as distinct escape sequences. We verify by confirming ANSI codes are present.
	results, names := makeResults("model", "Sonnet", "context", "42%")
	cfg := makeTestConfig(nil)

	line := renderPowerline(results, names, cfg)
	stripped := ansi.Strip(line)
	if line == stripped {
		t.Error("expected ANSI escape sequences in powerline output, found none")
	}
}

func TestRenderPowerline_FallbackBgWhenNoThemeEntry(t *testing.T) {
	// A widget that has no theme entry must use DefaultPowerlineBg as fallback.
	cfg := config.LoadHud()
	cfg.ResolvedTheme = make(theme.Theme) // empty theme
	config.ResolveTheme(cfg)
	// Manually clear the resolved theme to simulate a widget with no entry.
	cfg.ResolvedTheme = make(theme.Theme)

	r := widget.WidgetResult{Text: "test"}
	bg := resolveSegmentBg(r, "unknown-widget", cfg)
	if bg != theme.DefaultPowerlineBg {
		t.Errorf("expected fallback bg %q, got %q", theme.DefaultPowerlineBg, bg)
	}
}

func TestRenderPowerline_ThemeBgTakesPriorityOverDefault(t *testing.T) {
	// When a theme entry is set, it should be used instead of the fallback.
	t.Setenv("HOME", t.TempDir())
	cfg := config.LoadHud()
	cfg.Style.Theme = "dark"
	config.ResolveTheme(cfg)

	r := widget.WidgetResult{Text: "test"}
	bg := resolveSegmentBg(r, "model", cfg)
	// model bg in dark theme is "#2d2d2d" — not the default fallback "236"
	if bg == theme.DefaultPowerlineBg {
		t.Errorf("expected theme bg for 'model', got fallback %q", bg)
	}
	if bg != "#2d2d2d" {
		t.Errorf("expected model dark theme bg '#2d2d2d', got %q", bg)
	}
}

func TestRenderPowerline_WidgetBgOverridesTheme(t *testing.T) {
	// An explicit BgColor on the WidgetResult overrides the theme bg.
	cfg := config.LoadHud()
	cfg.Style.Theme = "dark"
	config.ResolveTheme(cfg)

	r := widget.WidgetResult{Text: "test", BgColor: "#ff0000"}
	bg := resolveSegmentBg(r, "model", cfg)
	if bg != "#ff0000" {
		t.Errorf("expected widget BgColor '#ff0000', got %q", bg)
	}
}

func TestRender_PowerlineModeIntegration(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Sonnet",
		ContextWindowSize: 200000,
		ContextPercent:    60,
		Cwd:               "/Users/kyle/Code/project",
	}
	cfg := makeTestConfig([]config.Line{
		{Widgets: []string{"model", "context"}, Mode: "powerline"},
	})

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if out == "" {
		t.Fatal("Render produced no output for powerline line")
	}
	stripped := ansi.Strip(out)
	if !strings.Contains(stripped, "Sonnet") {
		t.Errorf("expected 'Sonnet' in Render output, got %q", stripped)
	}
}

func TestRender_MixedModes(t *testing.T) {
	// A config with one powerline line and one plain line must render both.
	ctx := &model.RenderContext{
		ModelDisplayName:  "Opus",
		ContextWindowSize: 200000,
		ContextPercent:    80,
		Cwd:               "/Users/kyle/Code/project",
	}
	cfg := makeTestConfig([]config.Line{
		{Widgets: []string{"model"}, Mode: "powerline"},
		{Widgets: []string{"context"}},
	})

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 output lines, got %d: %q", len(lines), out)
	}
	if !strings.Contains(ansi.Strip(lines[0]), "Opus") {
		t.Errorf("expected 'Opus' in powerline line, got %q", lines[0])
	}
	if !strings.Contains(ansi.Strip(lines[1]), "80%") {
		t.Errorf("expected '80%%' in plain line, got %q", lines[1])
	}
}

func TestPowerlinePreset_TwoLines(t *testing.T) {
	// The powerline preset must have 2 lines, with line 1 in powerline mode.
	p, ok := preset.Load("powerline")
	if !ok {
		t.Fatal("powerline preset not found in registry")
	}
	if len(p.Lines) != 2 {
		t.Errorf("expected powerline preset to have 2 lines, got %d", len(p.Lines))
	}
	if p.Lines[0].Mode != "powerline" {
		t.Errorf("expected line 1 to have Mode='powerline', got %q", p.Lines[0].Mode)
	}
	if p.Lines[1].Mode == "powerline" {
		t.Errorf("expected line 2 to NOT be powerline mode, got %q", p.Lines[1].Mode)
	}
}

func TestPowerlinePreset_DarkTheme(t *testing.T) {
	p, ok := preset.Load("powerline")
	if !ok {
		t.Fatal("powerline preset not found")
	}
	if p.Theme != "dark" {
		t.Errorf("expected powerline preset to use 'dark' theme, got %q", p.Theme)
	}
}

func TestDarkTheme_DistinctBgColors(t *testing.T) {
	// The dark theme must provide distinct bg colors for the key powerline widgets.
	darkTheme := theme.Load("dark")
	widgets := []string{"model", "context", "project", "duration", "cost", "tools"}
	bgs := make(map[string]string)
	for _, w := range widgets {
		colors, ok := darkTheme[w]
		if !ok {
			t.Errorf("dark theme missing entry for widget %q", w)
			continue
		}
		if colors.Bg == "" {
			t.Errorf("dark theme has empty Bg for widget %q", w)
			continue
		}
		for prev, prevBg := range bgs {
			if prevBg == colors.Bg {
				t.Errorf("dark theme: %q and %q share the same bg color %q — they must be distinct for visible powerline transitions", prev, w, prevBg)
			}
		}
		bgs[w] = colors.Bg
	}
}
