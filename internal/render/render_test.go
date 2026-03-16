package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
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
	// When TerminalWidth is 0, output should not be truncated regardless of length.
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
		t.Errorf("expected full name in output when no width limit, got %q", line)
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
		t.Errorf("expected ' :: ' separator in output, got %q", out)
	}
}
