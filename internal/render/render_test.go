package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/theme"
)

func TestRender_ProducesOutput(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Sonnet",
		ContextWindowSize: 200000,
		ContextPercent:    50,
		Cwd:               "/Users/kyle/Code/project",
	}
	cfg := config.LoadHud()

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if out == "" {
		t.Fatal("Render produced no output")
	}

	// Default config line 1 has model, context, directory
	if !strings.Contains(out, "Sonnet") {
		t.Errorf("expected 'Sonnet' in output, got %q", out)
	}
	if !strings.Contains(out, "50%") {
		t.Errorf("expected '50%%' in output, got %q", out)
	}
	if !strings.Contains(out, "project") {
		t.Errorf("expected 'project' in output, got %q", out)
	}
}

func TestRender_SkipsUnknownWidgets(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Opus"}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"nonexistent", "model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if !strings.Contains(out, "Opus") {
		t.Errorf("expected 'Opus' after skipping unknown widget, got %q", out)
	}
}

func TestRender_SkipsEmptyLines(t *testing.T) {
	ctx := &model.RenderContext{} // no data -> all widgets return ""
	cfg := config.LoadHud()

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	// With no transcript or model data all widgets return "". All three lines
	// are empty and skipped, so output should be empty.
	out := buf.String()
	if out != "" {
		t.Errorf("expected empty output when all widgets have no data, got %q", out)
	}
}

func TestRender_TruncatesLongLines(t *testing.T) {
	// Build a model name long enough that the rendered line will exceed a narrow width.
	longName := strings.Repeat("X", 100)
	ctx := &model.RenderContext{
		ModelDisplayName: longName,
		TerminalWidth:    20,
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	line := strings.TrimRight(buf.String(), "\n")
	// Visual width must not exceed TerminalWidth.
	w := ansi.StringWidth(line)
	if w > 20 {
		t.Errorf("expected visual width <= 20, got %d for %q", w, line)
	}
	// Truncated lines must contain the suffix (ANSI reset codes may follow it).
	if !strings.Contains(line, truncateSuffix) {
		t.Errorf("expected %q in truncated line, got %q", truncateSuffix, line)
	}
}

func TestRender_NoTruncationWhenWidthZero(t *testing.T) {
	// When TerminalWidth is 0 (unknown), output passes through without
	// truncation — let Claude Code handle it. Matches claude-hud behavior.
	longName := strings.Repeat("Y", 200)
	ctx := &model.RenderContext{
		ModelDisplayName: longName,
		TerminalWidth:    0,
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	line := strings.TrimRight(buf.String(), "\n")
	if !strings.Contains(line, longName) {
		t.Errorf("expected full name in output when width unknown, got %q", line)
	}
}

func TestRender_NoTruncationWhenLineShortEnough(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName: "Sonnet",
		TerminalWidth:    200,
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	line := strings.TrimRight(buf.String(), "\n")
	if strings.HasSuffix(line, truncateSuffix) {
		t.Errorf("expected no truncation for short line, got %q", line)
	}
	if !strings.Contains(line, "Sonnet") {
		t.Errorf("expected 'Sonnet' in untruncated output, got %q", line)
	}
}

func TestRender_NoTruncationBelowMinWidth(t *testing.T) {
	// When TerminalWidth is below the minimum (20), truncation is skipped
	// entirely so the HUD does not collapse to "..." in very narrow terminals.
	ctx := &model.RenderContext{
		ModelDisplayName: "claude-sonnet-4-5",
		TerminalWidth:    10, // below minTruncateWidth
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	line := strings.TrimRight(buf.String(), "\n")
	// Line must not be just "..." — some real content must appear.
	if line == truncateSuffix {
		t.Errorf("expected real content at narrow width, got only %q", line)
	}
	// The full model name may or may not fit, but the suffix must NOT appear
	// since truncation is skipped below minTruncateWidth.
	if strings.HasSuffix(line, truncateSuffix) {
		t.Errorf("expected no truncation suffix below min width, got %q", line)
	}
}

func TestRender_AnsiResetPrefixPerLine(t *testing.T) {
	// Each rendered line must start with the ANSI reset escape so that Claude
	// Code's dim styling does not bleed into our statusline colors.
	ctx := &model.RenderContext{
		ModelDisplayName:  "Sonnet",
		ContextWindowSize: 200000,
		ContextPercent:    50,
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
		{Widgets: []string{"context"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("Render produced no lines")
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "\x1b[0m") {
			t.Errorf("line %d does not start with ANSI reset, got %q", i, line)
		}
	}
}

func TestRender_UsesSeparator(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Opus",
		ContextWindowSize: 200000,
		ContextPercent:    42,
	}
	cfg := config.LoadHud()
	cfg.Style.Separator = " :: "
	cfg.Lines = []config.Line{
		{Widgets: []string{"model", "context"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if !strings.Contains(out, " :: ") {
		t.Errorf("expected separator %q in output, got %q", " :: ", out)
	}
}

// TestRender_PlainModeOutputIdentical verifies that when a theme fg override
// exists, plain mode re-renders from PlainText with the override color.
// The default built-in theme assigns fg "135" to the env widget, so the
// pre-styled MutedStyle text is replaced with PlainText rendered in fg 135.
func TestRender_PlainModeOutputIdentical(t *testing.T) {
	ctx := &model.RenderContext{
		EnvCounts: &model.EnvCounts{MCPServers: 3, Hooks: 2},
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"env"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	rendered := strings.TrimRight(buf.String(), "\n")

	// Default theme has env fg = "135". Plain mode now re-renders PlainText
	// ("3M 2H") with that fg override instead of passing through MutedStyle.
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("135")).Render("3M 2H")
	want := "\x1b[0m" + styled + "\x1b[0m\x1b[K"
	if rendered != want {
		t.Errorf("plain mode output mismatch: got %q, want %q", rendered, want)
	}

	// Cross-check: verify the WidgetResult fields are still correct.
	result := widget.Registry["env"](ctx, cfg)
	if result.FgColor != "8" {
		t.Errorf("Env FgColor: expected '8', got %q", result.FgColor)
	}
	if result.PlainText != "3M 2H" {
		t.Errorf("Env PlainText: expected '3M 2H', got %q", result.PlainText)
	}
	mutedStyled := widget.MutedStyle.Render("3M 2H")
	if result.Text != mutedStyled {
		t.Errorf("Env Text: expected pre-styled %q, got %q", mutedStyled, result.Text)
	}
}

// --- Powerline renderer unit tests ---

// TestRenderPowerline_TwoSegmentTransition verifies that the arrow between two
// segments uses the left segment's bg as fg and the right segment's bg as bg.
func TestRenderPowerline_TwoSegmentTransition(t *testing.T) {
	segs := []widget.WidgetResult{
		{Text: "A", FgColor: "255", BgColor: "75"},
		{Text: "B", FgColor: "255", BgColor: "114"},
	}
	names := []string{"widget-a", "widget-b"}
	cfg := config.LoadHud()

	out := renderPowerline(segs, names, cfg)

	// Arrow character must be present.
	if !strings.Contains(out, powerlineArrow) {
		t.Errorf("expected powerline arrow character in output, got %q", out)
	}
	// Start cap must be present.
	if !strings.Contains(out, powerlineStartCap) {
		t.Errorf("expected start cap character in output, got %q", out)
	}
	// Both segment texts must appear.
	stripped := ansi.Strip(out)
	if !strings.Contains(stripped, "A") {
		t.Errorf("expected segment text 'A' in output, got %q", stripped)
	}
	if !strings.Contains(stripped, "B") {
		t.Errorf("expected segment text 'B' in output, got %q", stripped)
	}
	// Output must contain ANSI escape sequences for coloring.
	if out == stripped {
		t.Errorf("expected ANSI escape sequences in powerline output")
	}
}

// TestRenderPowerline_EmptySegmentsSkipped verifies that segments with empty
// Text are not rendered and do not produce spurious arrows.
func TestRenderPowerline_EmptySegmentsSkipped(t *testing.T) {
	segs := []widget.WidgetResult{
		{Text: "A", BgColor: "75"},
		{Text: "", BgColor: "114"}, // empty — must be skipped
		{Text: "C", BgColor: "196"},
	}
	names := []string{"widget-a", "widget-empty", "widget-c"}
	cfg := config.LoadHud()

	out := renderPowerline(segs, names, cfg)

	// A and C must still appear; empty segment must be skipped.
	stripped := ansi.Strip(out)
	if !strings.Contains(stripped, "A") {
		t.Errorf("expected 'A' in output, got %q", stripped)
	}
	if !strings.Contains(stripped, "C") {
		t.Errorf("expected 'C' in output, got %q", stripped)
	}
}

// TestRenderPowerline_SingleSegment verifies that a single segment renders with
// a start cap and an end cap arrow, but no mid-segment transition arrows.
func TestRenderPowerline_SingleSegment(t *testing.T) {
	segs := []widget.WidgetResult{
		{Text: "solo", FgColor: "255", BgColor: "75"},
	}
	names := []string{"widget-solo"}
	cfg := config.LoadHud()

	out := renderPowerline(segs, names, cfg)

	if !strings.Contains(out, powerlineStartCap) {
		t.Errorf("expected start cap in single-segment output, got %q", out)
	}
	if !strings.Contains(out, powerlineArrow) {
		t.Errorf("expected end cap arrow in single-segment output, got %q", out)
	}
	if !strings.Contains(out, "solo") {
		t.Errorf("expected segment text in single-segment output, got %q", out)
	}
	// Count arrow characters: start cap (×1) + end arrow (×1) = 2 total
	// but they use different codepoints, so just verify both appear once each.
	capCount := strings.Count(out, powerlineStartCap)
	arrowCount := strings.Count(out, powerlineArrow)
	if capCount != 1 {
		t.Errorf("expected exactly 1 start cap, got %d in %q", capCount, out)
	}
	if arrowCount != 1 {
		t.Errorf("expected exactly 1 end arrow, got %d in %q", arrowCount, out)
	}
}

// TestRenderPowerline_AllEmptyReturnsEmpty verifies that all-empty results
// produce an empty string (so the line is skipped).
func TestRenderPowerline_AllEmptyReturnsEmpty(t *testing.T) {
	segs := []widget.WidgetResult{
		{Text: ""},
		{Text: ""},
	}
	names := []string{"widget-a", "widget-b"}
	cfg := config.LoadHud()
	out := renderPowerline(segs, names, cfg)
	if out != "" {
		t.Errorf("expected empty string for all-empty segments, got %q", out)
	}
}

// TestRenderPowerline_DefaultBgFallback verifies that segments without a BgColor
// and without a theme entry use the DefaultPowerlineBg as a last resort.
func TestRenderPowerline_DefaultBgFallback(t *testing.T) {
	segs := []widget.WidgetResult{
		{Text: "X"}, // no BgColor, no theme entry
	}
	names := []string{"unknown-widget-xyz"}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = make(theme.Theme) // empty theme — forces fallback

	// Resolve via helper to confirm DefaultPowerlineBg is returned.
	bg := resolveSegmentBg(segs[0], names[0], cfg)
	if bg != theme.DefaultPowerlineBg {
		t.Errorf("expected default bg %q, got %q", theme.DefaultPowerlineBg, bg)
	}

	// Also verify the segment renders without panicking.
	out := renderPowerline(segs, names, cfg)
	if out == "" {
		t.Errorf("expected non-empty output for segment with fallback bg")
	}
}

// TestRender_PowerlineMode verifies that the Render function routes to the
// powerline renderer when style.mode = "powerline".
func TestRender_PowerlineMode(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName: "Sonnet",
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "powerline"
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// Powerline output must contain the start cap or the arrow — plain mode would not.
	if !strings.Contains(out, powerlineArrow) && !strings.Contains(out, powerlineStartCap) {
		t.Errorf("expected powerline characters in powerline mode output, got %q", out)
	}
	if !strings.Contains(out, "Sonnet") {
		t.Errorf("expected 'Sonnet' in powerline output, got %q", out)
	}
}

// TestRender_PerLinePowerlineMode verifies that a per-line mode override works
// independently of the global style.mode.
func TestRender_PerLinePowerlineMode(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Opus",
		ContextWindowSize: 200000,
		ContextPercent:    42,
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "plain" // global: plain
	cfg.Style.Separator = " | "
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}, Mode: "powerline"}, // line 0: powerline override
		{Widgets: []string{"context"}},                  // line 1: inherits plain
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), out)
	}

	// Line 0 (model, powerline mode) must have powerline characters.
	if !strings.Contains(lines[0], powerlineArrow) && !strings.Contains(lines[0], powerlineStartCap) {
		t.Errorf("line 0 (powerline): expected powerline chars, got %q", lines[0])
	}
	// Line 1 (context, plain mode) must not have start cap character.
	if strings.Contains(lines[1], powerlineStartCap) {
		t.Errorf("line 1 (plain): unexpected start cap char, got %q", lines[1])
	}
}

// TestRender_CapsuleModeFallsToPlain verifies that setting mode = "capsule"
// falls through to plain mode (the default case) rather than producing an error
// or empty output. Capsule mode was removed; existing configs using it should
// still render correctly.
func TestRender_CapsuleModeFallsToPlain(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Opus",
		ContextWindowSize: 200000,
		ContextPercent:    42,
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "capsule" // unknown mode — falls through to plain
	cfg.Style.Separator = " | "
	cfg.Lines = []config.Line{
		{Widgets: []string{"model", "context"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// Plain mode output must still contain both widget values.
	if !strings.Contains(out, "Opus") {
		t.Errorf("expected 'Opus' in plain fallback output, got %q", out)
	}
	if !strings.Contains(out, "42%") {
		t.Errorf("expected '42%%' in plain fallback output, got %q", out)
	}
	// Plain mode uses the separator between widgets.
	if !strings.Contains(out, " | ") {
		t.Errorf("expected separator in plain fallback output, got %q", out)
	}
}

func TestRender_MinimalMode(t *testing.T) {
	ctx := &model.RenderContext{
		ModelDisplayName:  "Haiku",
		ContextWindowSize: 200000,
		ContextPercent:    10,
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "minimal"
	cfg.Lines = []config.Line{
		{Widgets: []string{"model", "context"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// Minimal mode must produce output with both widgets.
	if !strings.Contains(out, "Haiku") {
		t.Errorf("expected 'Haiku' in minimal output, got %q", out)
	}
	if !strings.Contains(out, "10%") {
		t.Errorf("expected '10%%' in minimal output, got %q", out)
	}
}

func TestRender_MinimalMode_NoSeparator(t *testing.T) {
	// Minimal mode uses a single space, not the configured separator.
	ctx := &model.RenderContext{
		ModelDisplayName:  "Haiku",
		ContextWindowSize: 200000,
		ContextPercent:    10,
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "minimal"
	cfg.Style.Separator = " || "
	cfg.Lines = []config.Line{
		{Widgets: []string{"model", "context"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if strings.Contains(out, " || ") {
		t.Errorf("minimal mode must not use separator, got %q", out)
	}
}

func TestRender_PerLineMode(t *testing.T) {
	// A per-line mode override takes precedence over the global style mode.
	ctx := &model.RenderContext{
		ModelDisplayName:  "Sonnet",
		ContextWindowSize: 200000,
		ContextPercent:    75,
	}
	cfg := config.LoadHud()
	cfg.Style.Mode = "plain"
	cfg.Style.Separator = " || "
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}, Mode: "powerline"},
		{Widgets: []string{"context"}, Mode: "minimal"},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// First line (powerline) must have powerline arrows.
	if !strings.Contains(out, powerlineArrow) && !strings.Contains(out, powerlineStartCap) {
		t.Errorf("per-line powerline override: expected powerline chars in output, got %q", out)
	}
	// The separator is from plain mode and must not appear since both lines use overrides.
	if strings.Contains(out, " || ") {
		t.Errorf("per-line overrides must not use plain separator, got %q", out)
	}
}

// TestRenderMinimal_PreStyledPassthrough verifies that when FgColor is empty
// (pre-styled widget), renderMinimal passes the text through without applying
// theme fg, preventing double-wrapped ANSI escape codes.
func TestRenderMinimal_PreStyledPassthrough(t *testing.T) {
	// Simulate a pre-styled widget: Text contains embedded ANSI codes, FgColor == "".
	preStyledText := "\x1b[38;5;87mhello\x1b[0m"
	results := []widget.WidgetResult{
		{Text: preStyledText, FgColor: ""},
	}

	// Provide a theme entry for a widget named "mywidget" with fg "75".
	// renderMinimal must NOT apply this theme fg when FgColor is empty.
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{
		"mywidget": {Fg: "75"},
	}
	line := config.Line{Widgets: []string{"mywidget"}}

	out := renderMinimal(results, line, cfg)

	// The output must be exactly the pre-styled text — no lipgloss wrapping.
	if out != preStyledText {
		t.Errorf("expected pre-styled text passed through unchanged, got %q", out)
	}
	// Verify no additional fg=75 escape code was prepended.
	if strings.Contains(out, "\x1b[38;5;75m") {
		t.Errorf("theme fg color must not be applied to pre-styled widget (FgColor==''), got %q", out)
	}
}

// TestRenderMinimal_StructuredFgApplied verifies that when FgColor is non-empty
// (structured output), renderMinimal applies that fg color via lipgloss.
func TestRenderMinimal_StructuredFgApplied(t *testing.T) {
	results := []widget.WidgetResult{
		{Text: "hello", FgColor: "75"},
	}
	cfg := config.LoadHud()
	line := config.Line{Widgets: []string{"model"}}

	out := renderMinimal(results, line, cfg)

	// The output must contain the fg=75 escape sequence.
	if !strings.Contains(out, "\x1b[38;5;75m") {
		t.Errorf("expected fg=75 applied to structured widget output, got %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
}

// TestRenderMinimal_EmptyFgNoTheme verifies that when FgColor is empty and no
// theme entry exists, the text is passed through as-is.
func TestRenderMinimal_EmptyFgNoTheme(t *testing.T) {
	plainText := "plain text"
	results := []widget.WidgetResult{
		{Text: plainText, FgColor: ""},
	}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{} // empty theme
	line := config.Line{Widgets: []string{"model"}}

	out := renderMinimal(results, line, cfg)

	if out != plainText {
		t.Errorf("expected plain text passed through unchanged, got %q", out)
	}
}

func TestRender_ExtraOutputAppendsLine(t *testing.T) {
	// When ExtraOutput is set, Render must emit it as an additional line
	// after all configured widget lines.
	ctx := &model.RenderContext{
		ModelDisplayName: "Sonnet",
		ExtraOutput:      "my-custom-label",
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	if !strings.Contains(out, "my-custom-label") {
		t.Errorf("expected ExtraOutput label in render output, got %q", out)
	}
}

func TestRender_ExtraOutputAbsentWhenEmpty(t *testing.T) {
	// When ExtraOutput is empty, no extra line should be emitted.
	ctx := &model.RenderContext{
		ModelDisplayName: "Sonnet",
		ExtraOutput:      "",
	}
	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// With one configured line and no ExtraOutput, there should be exactly 1 line.
	if len(lines) != 1 {
		t.Errorf("expected 1 output line when ExtraOutput is empty, got %d: %q", len(lines), out)
	}
}

func TestApplyWidgetStyle_SkipsThemeFgWhenWidgetPreStyled(t *testing.T) {
	// When a widget returns FgColor="" (pre-styled), applyWidgetStyle
	// must pass r.Text through even when a theme fg override exists.
	r := widget.WidgetResult{
		Text:      "\x1b[33m | \x1b[m\x1b[2m | \x1b[m",
		PlainText: " |  | ",
		FgColor:   "",
	}

	cfg := &config.Config{}
	cfg.ResolvedTheme = theme.Theme{
		"tools": {Fg: "75", Bg: ""},
	}

	out := applyWidgetStyle(r, "tools", cfg)
	if out != r.Text {
		t.Errorf("expected pre-styled text preserved when FgColor is empty, got %q", out)
	}
}

func TestLineMode_Defaults(t *testing.T) {
	line := config.Line{Widgets: []string{"model"}}
	if got := lineMode(line, "powerline"); got != "powerline" {
		t.Errorf("expected global mode 'powerline', got %q", got)
	}
}

func TestLineMode_PerLineOverride(t *testing.T) {
	line := config.Line{Widgets: []string{"model"}, Mode: "minimal"}
	if got := lineMode(line, "powerline"); got != "minimal" {
		t.Errorf("expected per-line override 'minimal', got %q", got)
	}
}

func TestLineMode_FallbackToPlain(t *testing.T) {
	line := config.Line{Widgets: []string{"model"}}
	if got := lineMode(line, ""); got != "plain" {
		t.Errorf("expected fallback to 'plain', got %q", got)
	}
}

// --- Theme override tests ---

// TestApplyWidgetStyle_FgOverride verifies that when ResolvedTheme has an fg
// override, applyWidgetStyle re-renders from PlainText with the override color
// instead of using the pre-styled Text (which already has baked-in ANSI codes).
func TestApplyWidgetStyle_FgOverride(t *testing.T) {
	r := widget.WidgetResult{
		Text:      "\x1b[38;5;94msome model\x1b[0m", // pre-styled
		PlainText: "some model",
		FgColor:   "94",
	}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{
		"model": {Fg: "#ff8800"},
	}

	out := applyWidgetStyle(r, "model", cfg)

	// Must contain the 24-bit RGB color parameters for #ff8800 (r=255 g=136 b=0).
	// Lipgloss may combine fg+bg params in one SGR sequence, so check for the
	// color parameters rather than a standalone escape.
	if !strings.Contains(out, "38;2;255;136;0") {
		t.Errorf("expected override fg escape in output, got %q", out)
	}
	// Must contain the plain text.
	if !strings.Contains(out, "some model") {
		t.Errorf("expected 'some model' in output, got %q", out)
	}
	// Must NOT double-wrap the original fg=94 escape alongside the override.
	if strings.Contains(out, "\x1b[38;5;94m") {
		t.Errorf("expected original fg=94 to be absent when override is set, got %q", out)
	}
}

// TestApplyWidgetStyle_NoOverride verifies that without a theme override,
// applyWidgetStyle passes pre-styled Text through unchanged.
func TestApplyWidgetStyle_NoOverride(t *testing.T) {
	preStyled := "\x1b[38;5;94msome model\x1b[0m"
	r := widget.WidgetResult{
		Text:    preStyled,
		FgColor: "94",
	}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{} // no override

	out := applyWidgetStyle(r, "model", cfg)

	if out != preStyled {
		t.Errorf("expected pre-styled text unchanged, got %q", out)
	}
}

// TestApplyWidgetStyle_BgOverrideOnly verifies that a bg-only theme override
// applies the background while keeping the pre-styled Text intact.
func TestApplyWidgetStyle_BgOverrideOnly(t *testing.T) {
	preStyled := "\x1b[38;5;94msome model\x1b[0m"
	r := widget.WidgetResult{
		Text:    preStyled,
		FgColor: "94",
	}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{
		"model": {Bg: "236"},
	}

	out := applyWidgetStyle(r, "model", cfg)

	// Bg=236 must be applied (xterm-256 bg escape sequence).
	if !strings.Contains(out, "\x1b[48;5;236m") {
		t.Errorf("expected bg=236 in output, got %q", out)
	}
}

// TestResolveSegmentFg_ThemeOverridesWidget verifies that when the theme has an
// fg override for a widget, it wins over the widget's own FgColor default.
func TestResolveSegmentFg_ThemeOverridesWidget(t *testing.T) {
	r := widget.WidgetResult{FgColor: "94"} // model widget default
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{
		"model": {Fg: "#ff8800"},
	}

	got := resolveSegmentFg(r, "model", cfg)
	if got != "#ff8800" {
		t.Errorf("expected theme override '#ff8800', got %q", got)
	}
}

// TestResolveSegmentFg_FallsBackToWidgetFg verifies that without a theme
// override, resolveSegmentFg returns the widget's own FgColor.
func TestResolveSegmentFg_FallsBackToWidgetFg(t *testing.T) {
	r := widget.WidgetResult{FgColor: "94"}
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{} // no override

	got := resolveSegmentFg(r, "model", cfg)
	if got != "94" {
		t.Errorf("expected widget fg '94', got %q", got)
	}
}

// TestRender_PlainModeThemeFgOverride verifies end-to-end that a theme fg
// override applies in plain mode via Render, producing the override color escape.
func TestRender_PlainModeThemeFgOverride(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Sonnet"}
	cfg := config.LoadHud()
	cfg.Style.Mode = "plain"
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}
	cfg.ResolvedTheme = theme.Theme{
		"model": {Fg: "#ff8800"},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// 24-bit RGB color parameters for #ff8800 (r=255, g=136, b=0).
	if !strings.Contains(out, "38;2;255;136;0") {
		t.Errorf("expected override fg #ff8800 in plain mode output, got %q", out)
	}
	if !strings.Contains(out, "Sonnet") {
		t.Errorf("expected 'Sonnet' in plain mode output, got %q", out)
	}
}

// TestRender_PowerlineThemeFgOverride verifies end-to-end that a theme fg
// override wins over the widget's FgColor in powerline mode.
func TestRender_PowerlineThemeFgOverride(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Sonnet"}
	cfg := config.LoadHud()
	cfg.Style.Mode = "powerline"
	cfg.Lines = []config.Line{
		{Widgets: []string{"model"}},
	}
	cfg.ResolvedTheme = theme.Theme{
		"model": {Fg: "#ff8800"},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)

	out := buf.String()
	// 24-bit RGB color parameters for #ff8800 must appear in powerline output.
	// Lipgloss may combine fg+bg params in one SGR sequence, so check for the
	// color parameters rather than a standalone escape.
	if !strings.Contains(out, "38;2;255;136;0") {
		t.Errorf("expected override fg #ff8800 in powerline output, got %q", out)
	}
	if !strings.Contains(out, "Sonnet") {
		t.Errorf("expected 'Sonnet' in powerline output, got %q", out)
	}
}

// TestRender_ColorLevelWiresLipglossProfile verifies that Render() applies
// cfg.Style.ColorLevel to lipgloss.Writer.Profile so that color downsampling
// reflects the user's explicit preference rather than auto-detection from the
// pipe environment (where no TTY is present).
func TestRender_ColorLevelWiresLipglossProfile(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Sonnet"}

	tests := []struct {
		colorLevel string
		want       colorprofile.Profile
	}{
		{"truecolor", colorprofile.TrueColor},
		{"256", colorprofile.ANSI256},
		{"basic", colorprofile.ANSI},
	}

	for _, tc := range tests {
		t.Run(tc.colorLevel, func(t *testing.T) {
			cfg := config.LoadHud()
			cfg.Style.ColorLevel = tc.colorLevel
			cfg.Lines = []config.Line{
				{Widgets: []string{"model"}},
			}

			var buf bytes.Buffer
			Render(&buf, ctx, cfg)

			if lipgloss.Writer.Profile != tc.want {
				t.Errorf("color_level=%q: lipgloss.Writer.Profile = %v, want %v",
					tc.colorLevel, lipgloss.Writer.Profile, tc.want)
			}
		})
	}
}

// TestRender_ToolsDividerSurvivesPlainMode verifies that the tools widget's
// internal ANSI styling survives the plain-mode render pipeline.
//
// The bug was: applyWidgetStyle re-rendered from PlainText with a flat theme
// color, destroying per-element styling (yellow highlight separator, dim
// separators, green icons). Since the tools widget returns FgColor="" (pre-styled),
// applyWidgetStyle must pass Text through unchanged, preserving internal codes.
func TestRender_ToolsDividerSurvivesPlainMode(t *testing.T) {
	ctx := &model.RenderContext{
		TerminalWidth: 200,
		Transcript: &model.TranscriptData{
			Tools: []model.ToolEntry{
				{Name: "Read", Completed: true, DurationMs: 500, Category: "file"},
				{Name: "Write", Completed: true, DurationMs: 1200, Category: "file"},
				{Name: "Bash", Completed: true, DurationMs: 3000, Category: "shell"},
			},
			DividerOffset: 1,
		},
	}

	cfg := config.LoadHud()
	cfg.Lines = []config.Line{
		{Widgets: []string{"tools"}},
	}
	cfg.Style.Mode = "plain"

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)
	out := buf.String()

	// The output must contain ANSI color 33 (yellow) for the highlighted separator.
	// yellowStyle uses lipgloss.Color("3") which emits "[33m" in an ANSI terminal.
	if !strings.Contains(out, "[33m") && !strings.Contains(out, ";33m") {
		t.Errorf("expected yellow (ANSI 33) highlight separator in output, got %q", out)
	}

	// The output must also contain the faint/dim attribute ("[2m" or "[2;")
	// for the non-highlighted separators.
	if !strings.Contains(out, "[2m") && !strings.Contains(out, "[2;") {
		t.Errorf("expected dim/faint separator in output, got %q", out)
	}

	// Count distinct ANSI escape sequences -- there should be more than 4
	// (one open + one reset would indicate flat coloring from re-rendering PlainText).
	ansiCount := strings.Count(out, "[")
	if ansiCount <= 4 {
		t.Errorf("expected multiple distinct ANSI codes (got %d), output may be flat-colored: %q", ansiCount, out)
	}
}

// TestRender_CompletedAgentWithDurationVisible verifies that a completed agent
// with DurationMs >= 1000 appears in rendered output.
//
// The bug was: agents had DurationMs of 3-10ms (from the transcript extractor)
// and were filtered out by the >= 1000 threshold. Agents discovered from the
// filesystem now carry real durations (5-60s) and must pass the filter.
func TestRender_CompletedAgentWithDurationVisible(t *testing.T) {
	ctx := &model.RenderContext{
		TerminalWidth: 200,
		Transcript: &model.TranscriptData{
			Agents: []model.AgentEntry{
				{
					Name:       "Explore",
					Status:     "completed",
					DurationMs: 15000, // 15 seconds -- passes the >= 1000 filter
					StartTime:  time.Now().Add(-15 * time.Second),
					ColorIndex: 0,
				},
			},
		},
	}

	cfg := &config.Config{}
	cfg.Lines = []config.Line{
		{Widgets: []string{"agents"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)
	out := buf.String()

	if !strings.Contains(out, "Explore") {
		t.Errorf("expected completed agent 'Explore' with DurationMs=15000 to appear, got %q", out)
	}
}

// TestRender_CompletedAgentWithLowDurationHidden verifies that completed agents
// with DurationMs < 1000 do not appear in rendered output.
//
// Sub-second agents are noise -- they completed before the user could notice
// them. The agents widget filters them at the >= 1000 threshold.
func TestRender_CompletedAgentWithLowDurationHidden(t *testing.T) {
	ctx := &model.RenderContext{
		TerminalWidth: 200,
		Transcript: &model.TranscriptData{
			Agents: []model.AgentEntry{
				{
					Name:       "Ghost",
					Status:     "completed",
					DurationMs: 6, // 6ms -- filtered out
					StartTime:  time.Now().Add(-6 * time.Millisecond),
					ColorIndex: 0,
				},
			},
		},
	}

	cfg := &config.Config{}
	cfg.Lines = []config.Line{
		{Widgets: []string{"agents"}},
	}

	var buf bytes.Buffer
	Render(&buf, ctx, cfg)
	out := buf.String()

	if strings.Contains(out, "Ghost") {
		t.Errorf("expected agent 'Ghost' with DurationMs=6 to be hidden, but found in %q", out)
	}
}
