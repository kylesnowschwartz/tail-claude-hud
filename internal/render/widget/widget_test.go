package widget

import (
	"os"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func defaultCfg() *config.Config {
	return config.LoadHud()
}

func TestModelWidget_DisplaysNameInBrackets(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Opus", ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	if !strings.Contains(got, "Opus") {
		t.Errorf("expected output to contain 'Opus', got %q", got)
	}
	// Context size is no longer shown in the model widget.
	if strings.Contains(got, "context") {
		t.Errorf("expected no context size in model widget, got %q", got)
	}
}

func TestModelWidget_NeverShowsContextSize(t *testing.T) {
	// Context size is removed from the model widget entirely.
	ctx := &model.RenderContext{ModelDisplayName: "Sonnet", ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	if strings.Contains(got, "context") {
		t.Errorf("expected no context size in model widget, got %q", got)
	}
	if !strings.Contains(got, "Sonnet") {
		t.Errorf("expected 'Sonnet' in output, got %q", got)
	}
}

func TestModelWidget_EmptyName(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Model(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty result for empty model name, got %q", got.Text)
	}
}

func TestContextWidget_GreenUnder70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "50%") {
		t.Errorf("expected '50%%' in output, got %q", got)
	}
}

func TestContextWidget_YellowAt70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 75, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "75%") {
		t.Errorf("expected '75%%' in output, got %q", got)
	}
}

func TestContextWidget_RedAt85(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 90, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "90%") {
		t.Errorf("expected '90%%' in output, got %q", got)
	}
}

func TestContextWidget_EmptyWhenZero(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Context(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty result for zero context, got %q", got.Text)
	}
}

func TestContextWidget_PercentMode(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 42, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Value = "percent"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "42%") {
		t.Errorf("percent mode: expected '42%%' in output, got %q", got)
	}
}

func TestContextWidget_PercentModeDefault(t *testing.T) {
	// Empty Value string should behave like "percent".
	ctx := &model.RenderContext{ContextPercent: 55, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Value = ""

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "55%") {
		t.Errorf("empty value mode: expected '55%%' in output, got %q", got)
	}
}

func TestContextWidget_TokensMode(t *testing.T) {
	ctx := &model.RenderContext{
		ContextPercent:    42,
		ContextWindowSize: 200000,
		InputTokens:       80000,
		CacheCreation:     4000,
		CacheRead:         0,
	}
	cfg := defaultCfg()
	cfg.Context.Value = "tokens"

	got := Context(ctx, cfg).Text
	// used = 80000+4000 = 84000 → "84.0k", total = 200000 → "200k"
	if !strings.Contains(got, "84.0k") {
		t.Errorf("tokens mode: expected used '84.0k' in output, got %q", got)
	}
	if !strings.Contains(got, "200k") {
		t.Errorf("tokens mode: expected total '200k' in output, got %q", got)
	}
}

func TestContextWidget_RemainingMode(t *testing.T) {
	ctx := &model.RenderContext{
		ContextPercent:    42,
		ContextWindowSize: 200000,
		InputTokens:       80000,
		CacheCreation:     4000,
		CacheRead:         0,
	}
	cfg := defaultCfg()
	cfg.Context.Value = "remaining"

	got := Context(ctx, cfg).Text
	// remaining = 200000 - 84000 = 116000 → "116k"
	if !strings.Contains(got, "116k") {
		t.Errorf("remaining mode: expected '116k' in output, got %q", got)
	}
	if !strings.Contains(got, "left") {
		t.Errorf("remaining mode: expected 'left' in output, got %q", got)
	}
}

func TestContextWidget_BreakdownAppearsAbove85(t *testing.T) {
	ctx := &model.RenderContext{
		ContextPercent:    90,
		ContextWindowSize: 200000,
		InputTokens:       84000,
		CacheCreation:     12000,
		CacheRead:         8000,
	}
	cfg := defaultCfg()
	cfg.Context.ShowBreakdown = true

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "in:") {
		t.Errorf("breakdown: expected 'in:' in output, got %q", got)
	}
	if !strings.Contains(got, "cr:") {
		t.Errorf("breakdown: expected 'cr:' in output, got %q", got)
	}
	if !strings.Contains(got, "rd:") {
		t.Errorf("breakdown: expected 'rd:' in output, got %q", got)
	}
}

func TestContextWidget_BreakdownNotAppearsBelow85(t *testing.T) {
	ctx := &model.RenderContext{
		ContextPercent:    80,
		ContextWindowSize: 200000,
		InputTokens:       84000,
		CacheCreation:     12000,
		CacheRead:         8000,
	}
	cfg := defaultCfg()
	cfg.Context.ShowBreakdown = true

	got := Context(ctx, cfg).Text
	if strings.Contains(got, "in:") {
		t.Errorf("breakdown: expected no breakdown at 80%%, got %q", got)
	}
}

func TestContextWidget_BreakdownDisabled(t *testing.T) {
	ctx := &model.RenderContext{
		ContextPercent:    90,
		ContextWindowSize: 200000,
		InputTokens:       84000,
		CacheCreation:     12000,
		CacheRead:         8000,
	}
	cfg := defaultCfg()
	cfg.Context.ShowBreakdown = false

	got := Context(ctx, cfg).Text
	if strings.Contains(got, "in:") {
		t.Errorf("breakdown disabled: expected no breakdown, got %q", got)
	}
}

// TestContextWidget_NamedColorProducesANSICodes verifies that the default config
// color "green" (a CSS named color) produces ANSI escape codes in the output.
// Regression test for: lipgloss.Color("green") returning noColor and stripping
// all color from context widget output.
func TestContextWidget_NamedColorProducesANSICodes(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	// The default config uses "green" as the context color, which triggered the bug.
	cfg.Style.Colors.Context = "green"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("Context with named color 'green': expected ANSI escape codes in output, got %q", got)
	}
}

// -- renderBar ----------------------------------------------------------------

func TestRenderBar_ZeroPercent(t *testing.T) {
	got := renderBar(0, 10)
	want := "░░░░░░░░░░"
	if got != want {
		t.Errorf("renderBar(0, 10) = %q, want %q", got, want)
	}
}

func TestRenderBar_FullPercent(t *testing.T) {
	got := renderBar(100, 10)
	want := "██████████"
	if got != want {
		t.Errorf("renderBar(100, 10) = %q, want %q", got, want)
	}
}

func TestRenderBar_FiftyPercent(t *testing.T) {
	got := renderBar(50, 10)
	want := "█████░░░░░"
	if got != want {
		t.Errorf("renderBar(50, 10) = %q, want %q", got, want)
	}
}

func TestRenderBar_WidthFive(t *testing.T) {
	// 40% of width 5 = 2 filled
	got := renderBar(40, 5)
	want := "██░░░"
	if got != want {
		t.Errorf("renderBar(40, 5) = %q, want %q", got, want)
	}
}

func TestRenderBar_ClampsBelowZero(t *testing.T) {
	got := renderBar(-10, 10)
	want := "░░░░░░░░░░"
	if got != want {
		t.Errorf("renderBar(-10, 10) = %q, want %q", got, want)
	}
}

func TestRenderBar_ClampsAbove100(t *testing.T) {
	got := renderBar(150, 10)
	want := "██████████"
	if got != want {
		t.Errorf("renderBar(150, 10) = %q, want %q", got, want)
	}
}

// -- contextThresholds color boundary tests -----------------------------------

// Normal: below 60% → green (context color)
func TestContextThresholds_NormalBelow60(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	got := contextThresholds(59, 60, 80, green, yellow, red)
	if got.Render("x") != green.Render("x") {
		t.Errorf("contextThresholds(59): expected green, got different style")
	}
}

// Warning boundary: exactly 60% → yellow
func TestContextThresholds_WarningAt60(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	got := contextThresholds(60, 60, 80, green, yellow, red)
	if got.Render("x") != yellow.Render("x") {
		t.Errorf("contextThresholds(60): expected yellow (warning), got different style")
	}
}

// Still warning at 79%
func TestContextThresholds_WarningAt79(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	got := contextThresholds(79, 60, 80, green, yellow, red)
	if got.Render("x") != yellow.Render("x") {
		t.Errorf("contextThresholds(79): expected yellow (warning), got different style")
	}
}

// Critical boundary: exactly 80% → red
func TestContextThresholds_CriticalAt80(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	got := contextThresholds(80, 60, 80, green, yellow, red)
	if got.Render("x") != red.Render("x") {
		t.Errorf("contextThresholds(80): expected red (critical), got different style")
	}
}

// Critical well above threshold
func TestContextThresholds_CriticalAt100(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	got := contextThresholds(100, 60, 80, green, yellow, red)
	if got.Render("x") != red.Render("x") {
		t.Errorf("contextThresholds(100): expected red (critical), got different style")
	}
}

// -- Context widget display modes ---------------------------------------------

func TestContextWidget_BarMode_ContainsBlockChars(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "bar"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "█") {
		t.Errorf("bar mode: expected filled block char '█', got %q", got)
	}
	if !strings.Contains(got, "░") {
		t.Errorf("bar mode: expected empty block char '░', got %q", got)
	}
	// Bar mode must not show the percent label.
	if strings.Contains(got, "%") {
		t.Errorf("bar mode: expected no percent label, got %q", got)
	}
}

func TestContextWidget_BarMode_ZeroPercent(t *testing.T) {
	// Zero ContextPercent but non-zero window size should render an empty bar.
	ctx := &model.RenderContext{ContextPercent: 0, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "bar"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "░") {
		t.Errorf("bar mode 0%%: expected all-empty bar (░), got %q", got)
	}
	if strings.Contains(got, "█") {
		t.Errorf("bar mode 0%%: expected no filled block, got %q", got)
	}
}

func TestContextWidget_BarMode_FullPercent(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 100, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "bar"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "█") {
		t.Errorf("bar mode 100%%: expected filled bar (█), got %q", got)
	}
	if strings.Contains(got, "░") {
		t.Errorf("bar mode 100%%: expected no empty block, got %q", got)
	}
}

func TestContextWidget_BothMode_ContainsBarAndLabel(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 42, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "both"
	cfg.Context.Value = "percent"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "█") && !strings.Contains(got, "░") {
		t.Errorf("both mode: expected block chars, got %q", got)
	}
	if !strings.Contains(got, "42%") {
		t.Errorf("both mode: expected '42%%' label, got %q", got)
	}
}

func TestContextWidget_TextMode_NoBlockChars(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "text"

	got := Context(ctx, cfg).Text
	if strings.Contains(got, "█") || strings.Contains(got, "░") {
		t.Errorf("text mode: expected no block chars, got %q", got)
	}
	if !strings.Contains(got, "50%") {
		t.Errorf("text mode: expected '50%%', got %q", got)
	}
}

func TestContextWidget_DefaultDisplayIsText(t *testing.T) {
	// Empty Display string should behave like "text".
	ctx := &model.RenderContext{ContextPercent: 55, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = ""

	got := Context(ctx, cfg).Text
	if strings.Contains(got, "█") || strings.Contains(got, "░") {
		t.Errorf("empty display: expected text mode (no block chars), got %q", got)
	}
	if !strings.Contains(got, "55%") {
		t.Errorf("empty display: expected '55%%', got %q", got)
	}
}

func TestContextWidget_BarWidthConfigurable(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 100, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Display = "bar"
	cfg.Context.BarWidth = 5

	got := Context(ctx, cfg).Text
	// Strip ANSI sequences by checking raw rune count of block chars.
	filled := strings.Count(got, "█")
	empty := strings.Count(got, "░")
	if filled+empty != 5 {
		t.Errorf("bar width 5, 100%%: expected 5 block chars total, got %d filled + %d empty in %q", filled, empty, got)
	}
}

// -- formatTokenCount ---------------------------------------------------------

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{123, "123"},
		{999, "999"},
		{1000, "1.0k"},
		{12300, "12.3k"},
		{99999, "100.0k"},
		{100000, "100k"},
		{123456, "123k"},
		{200000, "200k"},
	}
	for _, tt := range tests {
		got := formatTokenCount(tt.n)
		if got != tt.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestDirectoryWidget_SingleSegment(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Levels = 1

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "tail-claude-hud") {
		t.Errorf("expected 'tail-claude-hud', got %q", got)
	}
}

func TestDirectoryWidget_MultipleSegments(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Levels = 2

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "my-projects/tail-claude-hud") {
		t.Errorf("expected 2 segments, got %q", got)
	}
}

func TestDirectoryWidget_EmptyCwd(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Directory(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty result for empty cwd, got %q", got.Text)
	}
}

func TestRegistryHasAllWidgets(t *testing.T) {
	expected := []string{"model", "context", "directory", "git", "project", "env", "duration", "tools", "agents", "todos", "session", "thinking", "tokens", "cost", "lines", "outputstyle", "messages", "skills", "speed", "permission", "usage", "worktree"}
	for _, name := range expected {
		if _, ok := Registry[name]; !ok {
			t.Errorf("Registry missing widget %q", name)
		}
	}
	if len(Registry) != len(expected) {
		t.Errorf("Registry has %d entries, expected %d", len(Registry), len(expected))
	}
}

func TestTranscriptWidgets_NilTranscriptReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: nil}
	cfg := defaultCfg()

	// All transcript-powered widgets must return empty when Transcript is nil.
	widgets := []string{"tools", "agents", "todos", "session", "thinking", "messages", "speed"}
	for _, name := range widgets {
		fn := Registry[name]
		if got := fn(ctx, cfg); !got.IsEmpty() {
			t.Errorf("widget %q with nil Transcript: expected empty, got %q", name, got.Text)
		}
	}
}

// -- Tools widget -------------------------------------------------------------

func TestToolsWidget_EmptyToolsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Tools(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Tools with empty tools: expected empty, got %q", got.Text)
	}
}

// Spec 8: running tool renders with category icon + name + elapsed.
func TestToolsWidget_RunningToolShowsCategoryIconAndName(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Read", Category: "Read"}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg).Text
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Read) {
		t.Errorf("Tools running: expected Read category icon %q, got %q", icons.Read, got)
	}
	if !strings.Contains(got, "Read") {
		t.Errorf("Tools running: expected tool name 'Read', got %q", got)
	}
}

func TestToolsWidget_RunningToolShowsYellowIconNoSpinner(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Bash", Category: "Bash"}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg).Text
	if !strings.Contains(got, "Bash") {
		t.Errorf("Running tool should contain name 'Bash', got %q", got)
	}
	// Tool output should not contain any braille characters.
	brailleChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, frame := range brailleChars {
		if strings.Contains(got, frame) {
			t.Errorf("Running tool should not have braille spinner, got %q", got)
			break
		}
	}
}

// Spec 9: completed tool renders with dim category icon + name + duration.
func TestToolsWidget_CompletedToolShowsDimCategoryIconAndDuration(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Write", Completed: true, Category: "Write", DurationMs: 300}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg).Text
	if !strings.Contains(got, "Write") {
		t.Errorf("Tools completed: expected tool name 'Write', got %q", got)
	}
	if !strings.Contains(got, "0.3s") {
		t.Errorf("Tools completed: expected duration '0.3s', got %q", got)
	}
	// No error icon should appear for a non-error entry.
	icons := IconsFor("ascii")
	if strings.Contains(got, icons.Error) {
		t.Errorf("Tools completed (no error): unexpected error icon in %q", got)
	}
}

// Spec 10: error tool renders with red error icon + name + "err".
func TestToolsWidget_ErrorToolShowsRedCategoryIcon(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Bash", Completed: true, Category: "Bash", DurationMs: 500, HasError: true}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg).Text
	icons := IconsFor("ascii")
	// Error uses category icon (not error icon) in red.
	if !strings.Contains(got, icons.Bash) {
		t.Errorf("Tools error: expected Bash icon %q, got %q", icons.Bash, got)
	}
	if !strings.Contains(got, "Bash") {
		t.Errorf("Tools error: expected tool name 'Bash', got %q", got)
	}
}

// Spec 11: max 5 tools shown.
func TestToolsWidget_MaxFiveToolsShown(t *testing.T) {
	// Six completed tools — only 5 should be rendered.
	tools := []model.ToolEntry{
		{Name: "T1", Completed: true, DurationMs: 100},
		{Name: "T2", Completed: true, DurationMs: 100},
		{Name: "T3", Completed: true, DurationMs: 100},
		{Name: "T4", Completed: true, DurationMs: 100},
		{Name: "T5", Completed: true, DurationMs: 100},
		{Name: "T6", Completed: true, DurationMs: 100},
	}
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{Tools: tools}}
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text
	// Count separator occurrences: 4 " | " means 5 parts.
	count := strings.Count(got, " | ")
	if count != 4 {
		t.Errorf("Tools max 5: expected 4 separators (5 items), got %d in %q", count, got)
	}
	// T1 is the oldest; with newest-first reversal, T6 is first and T2 is last visible.
	if strings.Contains(got, "T1") {
		t.Errorf("Tools max 5: oldest tool T1 should be excluded, got %q", got)
	}
}

// Spec 12: empty when no tools.
func TestToolsWidget_NilTranscriptReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: nil}
	cfg := defaultCfg()

	if got := Tools(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Tools nil transcript: expected empty, got %q", got.Text)
	}
}

// -- formatDuration -----------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{0, "0.0s"},
		{-100, "0.0s"},
		{1, "<0.1s"},
		{50, "<0.1s"},
		{99, "<0.1s"},
		{100, "0.1s"},
		{300, "0.3s"},
		{999, "0.9s"},
		{1000, "1s"},
		{1500, "1.5s"},
		{10000, "10s"},
		{12300, "12.3s"},
		{59999, "59.9s"},
		{60000, "1m 0s"},
		{90000, "1m 30s"},
		{3661000, "61m 1s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.ms)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

// -- Agents widget ------------------------------------------------------------

func TestAgentsWidget_EmptyAgentsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Agents(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Agents with empty agents: expected empty, got %q", got.Text)
	}
}

func TestAgentsWidget_RunningAgentShowsColoredIconAndRunningIndicator(t *testing.T) {
	startTime := time.Now().Add(-2*time.Minute - 15*time.Second)
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "explore", Status: "running", ColorIndex: 0, StartTime: startTime},
		},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Agents(ctx, cfg).Text

	// Agent name must appear.
	if !strings.Contains(got, "explore") {
		t.Errorf("Agents running: expected name 'explore', got %q", got)
	}
	// The static running indicator must appear (ascii mode uses "~").
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Running) {
		t.Errorf("Agents running: expected running indicator %q in output, got %q", icons.Running, got)
	}
	// Elapsed time should appear (at least minutes marker).
	if !strings.Contains(got, "m") {
		t.Errorf("Agents running: expected elapsed time with 'm', got %q", got)
	}
}

func TestAgentsWidget_RunningAgentUsesAgentIcon(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "TaskWorker", Status: "running", ColorIndex: 1, StartTime: time.Now()},
		},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Agents(ctx, cfg).Text
	icons := IconsFor("ascii")
	// The agent icon should be rendered (ASCII mode uses "@" for Task).
	if !strings.Contains(got, icons.Task) {
		t.Errorf("Agents running: expected agent icon %q, got %q", icons.Task, got)
	}
}

func TestAgentsWidget_CompletedAgentShowsCheck(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "SearchAgent", Status: "completed", ColorIndex: 2, DurationMs: 5000},
		},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Agents(ctx, cfg).Text
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Check) {
		t.Errorf("Agents completed: expected check icon %q, got %q", icons.Check, got)
	}
	if !strings.Contains(got, "SearchAgent") {
		t.Errorf("Agents completed: expected name 'SearchAgent', got %q", got)
	}
	// Duration should show "5s".
	if !strings.Contains(got, "5s") {
		t.Errorf("Agents completed: expected duration '5s', got %q", got)
	}
}

func TestAgentsWidget_DifferentAgentsGetDifferentColors(t *testing.T) {
	// Two agents with distinct ColorIndex values must render with distinct color styles.
	style0 := AgentColorStyle(0)
	style1 := AgentColorStyle(1)

	rendered0 := style0.Render("test")
	rendered1 := style1.Render("test")
	if rendered0 == rendered1 {
		t.Errorf("AgentColorStyle(0) and AgentColorStyle(1) produced identical rendering %q", rendered0)
	}
}

func TestAgentsWidget_ModelSuffixShownWhenPresent(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "explore", Status: "running", ColorIndex: 0, Model: "claude-haiku-4-5", StartTime: time.Now()},
		},
	}}
	cfg := defaultCfg()

	got := Agents(ctx, cfg).Text
	if !strings.Contains(got, "haiku") {
		t.Errorf("Agents: expected model family 'haiku' in output, got %q", got)
	}
}

func TestAgentsWidget_NoModelSuffixWhenAbsent(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "worker", Status: "running", ColorIndex: 0, Model: "", StartTime: time.Now()},
		},
	}}
	cfg := defaultCfg()

	got := Agents(ctx, cfg).Text
	if strings.Contains(got, "(") {
		t.Errorf("Agents: expected no model suffix when Model is empty, got %q", got)
	}
}

func TestAgentsWidget_MaxFiveTotal(t *testing.T) {
	// 4 running + 3 completed = 7 agents; should cap at 5.
	agents := []model.AgentEntry{
		{Name: "r1", Status: "running", ColorIndex: 0, StartTime: time.Now()},
		{Name: "r2", Status: "running", ColorIndex: 1, StartTime: time.Now()},
		{Name: "r3", Status: "running", ColorIndex: 2, StartTime: time.Now()},
		{Name: "r4", Status: "running", ColorIndex: 3, StartTime: time.Now()},
		{Name: "c1", Status: "completed", ColorIndex: 4, DurationMs: 1000},
		{Name: "c2", Status: "completed", ColorIndex: 5, DurationMs: 2000},
		{Name: "c3", Status: "completed", ColorIndex: 6, DurationMs: 3000},
	}
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{Agents: agents}}
	cfg := defaultCfg()

	got := Agents(ctx, cfg).Text
	// Count the " | " separators — 4 separators means 5 entries.
	separators := strings.Count(got, " | ")
	if separators > 4 {
		t.Errorf("Agents: expected at most 5 entries (4 separators), got %d separators in %q", separators, got)
	}
}

func TestAgentsWidget_TruncatesWithPlusMoreWhenNarrow(t *testing.T) {
	// With a narrow terminal width, agents that don't fit should be replaced
	// with a "+N more" indicator rather than being cut mid-character by the
	// render-level truncation.
	agents := []model.AgentEntry{
		{Name: "agent-alpha", Status: "running", ColorIndex: 0, StartTime: time.Now()},
		{Name: "agent-beta", Status: "running", ColorIndex: 1, StartTime: time.Now()},
		{Name: "agent-gamma", Status: "running", ColorIndex: 2, StartTime: time.Now()},
	}
	ctx := &model.RenderContext{
		Transcript:    &model.TranscriptData{Agents: agents},
		TerminalWidth: 40, // narrow enough that 3 agents don't fit
	}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Agents(ctx, cfg)

	// The plain text must contain "+N more" since 3 agents can't fit in 40 cols.
	if !strings.Contains(got.PlainText, "more") {
		t.Errorf("expected '+N more' in PlainText for narrow width, got %q", got.PlainText)
	}
	// The first agent must still appear.
	if !strings.Contains(got.PlainText, "agent-alpha") {
		t.Errorf("expected first agent 'agent-alpha' to remain, got %q", got.PlainText)
	}
}

func TestAgentsWidget_NoTruncationWhenWidthZero(t *testing.T) {
	// When TerminalWidth is 0 (unknown), the widget skips its own truncation
	// and relies on the render-stage fallback width. All agents must appear.
	agents := []model.AgentEntry{
		{Name: "alpha", Status: "running", ColorIndex: 0, StartTime: time.Now()},
		{Name: "beta", Status: "running", ColorIndex: 1, StartTime: time.Now()},
		{Name: "gamma", Status: "running", ColorIndex: 2, StartTime: time.Now()},
	}
	ctx := &model.RenderContext{
		Transcript:    &model.TranscriptData{Agents: agents},
		TerminalWidth: 0,
	}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Agents(ctx, cfg)

	// All three agents must appear in plain text.
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(got.PlainText, name) {
			t.Errorf("expected all agents when width=0, missing %q in %q", name, got.PlainText)
		}
	}
	// No "+N more" should be present since the widget skips truncation at width=0.
	if strings.Contains(got.PlainText, "more") {
		t.Errorf("expected no '+N more' when TerminalWidth=0, got %q", got.PlainText)
	}
}

func TestTruncateAgentEntries_AllFit(t *testing.T) {
	styled := []string{"A", "B"}
	plain := []string{"A", "B"}
	outStyled, outPlain := truncateAgentEntries(styled, plain, 200)
	if len(outPlain) != 2 || outPlain[0] != "A" || outPlain[1] != "B" {
		t.Errorf("all-fit: expected [A B], got %v", outPlain)
	}
	_ = outStyled
}

func TestTruncateAgentEntries_OneDropped(t *testing.T) {
	// Width must accommodate "agent-alpha 1m30s | +1 more" (28 chars)
	// plus the 8-column renderer overhead, so maxWidth=40 gives available=32.
	plain := []string{"agent-alpha 1m30s", "agent-beta 0m45s"}
	styled := plain // use same for simplicity
	outStyled, outPlain := truncateAgentEntries(styled, plain, 40)
	// Must have exactly 2 elements: first entry + "+1 more"
	if len(outPlain) != 2 {
		t.Errorf("expected 2 output elements (1 entry + indicator), got %d: %v", len(outPlain), outPlain)
	}
	if outPlain[0] != "agent-alpha 1m30s" {
		t.Errorf("expected first entry preserved, got %q", outPlain[0])
	}
	if outPlain[1] != "+1 more" {
		t.Errorf("expected '+1 more', got %q", outPlain[1])
	}
	_ = outStyled
}

// -- Todos widget -------------------------------------------------------------

func TestTodosWidget_EmptyTodosReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Todos(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Todos with empty list: expected empty, got %q", got.Text)
	}
}

func TestTodosWidget_AllDoneHides(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Todos: []model.TodoItem{
			{ID: "1", Content: "First", Done: true},
			{ID: "2", Content: "Second", Done: true},
		},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Todos(ctx, cfg)
	if got.Text != "" {
		t.Errorf("Todos all done: expected empty widget, got %q", got.Text)
	}
}

func TestTodosWidget_PartialDoneShowsCount(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Todos: []model.TodoItem{
			{ID: "1", Content: "First", Done: true},
			{ID: "2", Content: "Second", Done: false},
			{ID: "3", Content: "Third", Done: false},
		},
	}}
	cfg := defaultCfg()

	got := Todos(ctx, cfg).Text
	if !strings.Contains(got, "1/3") {
		t.Errorf("Todos partial: expected '1/3', got %q", got)
	}
}

func TestTodosWidget_NoneDoneShowsDimCount(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Todos: []model.TodoItem{
			{ID: "1", Content: "First", Done: false},
			{ID: "2", Content: "Second", Done: false},
		},
	}}
	cfg := defaultCfg()

	got := Todos(ctx, cfg).Text
	if !strings.Contains(got, "0/2") {
		t.Errorf("Todos none done: expected '0/2', got %q", got)
	}
}

// -- Env widget ---------------------------------------------------------------

func TestEnvWidget_NilEnvCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: nil}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Env with nil EnvCounts: expected empty, got %q", got.Text)
	}
}

func TestEnvWidget_ZeroCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{}}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Env with all-zero counts: expected empty, got %q", got.Text)
	}
}

func TestEnvWidget_CompactFormat_AllCategories(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{
		MCPServers:    3,
		ClaudeMdFiles: 2,
		RuleFiles:     4,
		Hooks:         1,
	}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	// Each category must appear with its letter suffix.
	for _, want := range []string{"3M", "2C", "4R", "1H"} {
		if !strings.Contains(got, want) {
			t.Errorf("Env compact format: expected %q in output, got %q", want, got)
		}
	}
}

func TestEnvWidget_SkipsZeroCategories(t *testing.T) {
	// Only MCPs and hooks are non-zero; C and R must not appear.
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{
		MCPServers: 5,
		Hooks:      2,
	}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	if !strings.Contains(got, "5M") {
		t.Errorf("Env: expected '5M', got %q", got)
	}
	if !strings.Contains(got, "2H") {
		t.Errorf("Env: expected '2H', got %q", got)
	}
	if strings.Contains(got, "C") {
		t.Errorf("Env: expected no 'C' when ClaudeMdFiles=0, got %q", got)
	}
	if strings.Contains(got, "R") {
		t.Errorf("Env: expected no 'R' when RuleFiles=0, got %q", got)
	}
}

func TestEnvWidget_MCPOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 3}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	if !strings.Contains(got, "3M") {
		t.Errorf("Env MCPOnly: expected '3M' in output, got %q", got)
	}
}

func TestEnvWidget_ClaudeMdOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{ClaudeMdFiles: 2}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	if !strings.Contains(got, "2C") {
		t.Errorf("Env ClaudeMdOnly: expected '2C' in output, got %q", got)
	}
}

func TestEnvWidget_RuleFilesOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{RuleFiles: 4}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	if !strings.Contains(got, "4R") {
		t.Errorf("Env RuleFilesOnly: expected '4R' in output, got %q", got)
	}
}

func TestEnvWidget_HooksOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{Hooks: 3}}
	cfg := defaultCfg()

	got := Env(ctx, cfg).Text
	if !strings.Contains(got, "3H") {
		t.Errorf("Env HooksOnly: expected '3H' in output, got %q", got)
	}
}

// -- Duration widget ----------------------------------------------------------

func TestDurationWidget_EmptySessionStartReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{SessionStart: ""}
	cfg := defaultCfg()

	if got := Duration(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Duration with empty SessionStart: expected empty, got %q", got.Text)
	}
}

func TestDurationWidget_RendersTimestamp(t *testing.T) {
	// Use a timestamp 2 hours and 30 minutes ago.
	start := time.Now().Add(-2*time.Hour - 30*time.Minute).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionStart: start}
	cfg := defaultCfg()

	got := Duration(ctx, cfg).Text
	if !strings.Contains(got, "h") {
		t.Errorf("Duration >= 1h: expected 'h' in output, got %q", got)
	}
	if !strings.Contains(got, "m") {
		t.Errorf("Duration >= 1h: expected 'm' in output, got %q", got)
	}
}

func TestDurationWidget_ShortSession(t *testing.T) {
	start := time.Now().Add(-5*time.Minute - 10*time.Second).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionStart: start}
	cfg := defaultCfg()

	got := Duration(ctx, cfg).Text
	if !strings.Contains(got, "m") {
		t.Errorf("Duration < 1h: expected 'm' in output, got %q", got)
	}
	if !strings.Contains(got, "s") {
		t.Errorf("Duration < 1h: expected 's' in output, got %q", got)
	}
}

func TestDurationWidget_UsesIconLookup(t *testing.T) {
	start := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionStart: start}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Duration(ctx, cfg).Text
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Clock) {
		t.Errorf("Duration(ascii): expected clock icon %q, got %q", icons.Clock, got)
	}
}

// -- Git widget ---------------------------------------------------------------

func TestGitWidget_NilGitReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Git: nil}
	cfg := defaultCfg()

	if got := Git(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Git with nil Git: expected empty, got %q", got.Text)
	}
}

func TestGitWidget_ShowsBranch(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()

	got := Git(ctx, cfg).Text
	if !strings.Contains(got, "main") {
		t.Errorf("Git: expected 'main' in output, got %q", got)
	}
}

func TestGitWidget_DirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/foo", Dirty: true}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg).Text
	if !strings.Contains(got, "*") {
		t.Errorf("Git dirty: expected '*' in output, got %q", got)
	}
}

func TestGitWidget_CleanNoDirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main", Dirty: false}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg).Text
	if strings.Contains(got, "*") {
		t.Errorf("Git clean: expected no '*', got %q", got)
	}
}

func TestGitWidget_AheadBehindCounts(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/bar", AheadBy: 2, BehindBy: 1}}
	cfg := defaultCfg()
	cfg.Git.AheadBehind = true

	got := Git(ctx, cfg).Text
	if !strings.Contains(got, "↑2") {
		t.Errorf("Git ahead: expected '↑2', got %q", got)
	}
	if !strings.Contains(got, "↓1") {
		t.Errorf("Git behind: expected '↓1', got %q", got)
	}
}

func TestGitWidget_UsesIconLookup(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Git(ctx, cfg).Text
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Branch) {
		t.Errorf("Git(ascii): expected branch icon %q, got %q", icons.Branch, got)
	}
}

func TestGitWidget_FileStats_ShowsCounts(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{
		Branch:    "main",
		Modified:  3,
		Staged:    1,
		Untracked: 2,
	}}
	cfg := defaultCfg()
	cfg.Git.FileStats = true

	result := Git(ctx, cfg)
	if !strings.Contains(result.Text, "~3") {
		t.Errorf("FileStats: expected '~3' in Text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "+1") {
		t.Errorf("FileStats: expected '+1' in Text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "?2") {
		t.Errorf("FileStats: expected '?2' in Text, got %q", result.Text)
	}
	if !strings.Contains(result.PlainText, "~3") {
		t.Errorf("FileStats: expected '~3' in PlainText, got %q", result.PlainText)
	}
	if !strings.Contains(result.PlainText, "+1") {
		t.Errorf("FileStats: expected '+1' in PlainText, got %q", result.PlainText)
	}
	if !strings.Contains(result.PlainText, "?2") {
		t.Errorf("FileStats: expected '?2' in PlainText, got %q", result.PlainText)
	}
}

func TestGitWidget_FileStats_HidesZeroCounts(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{
		Branch:   "main",
		Modified: 0,
		Staged:   2,
	}}
	cfg := defaultCfg()
	cfg.Git.FileStats = true

	result := Git(ctx, cfg)
	if strings.Contains(result.Text, "~") {
		t.Errorf("FileStats: expected no '~' when Modified=0, got %q", result.Text)
	}
	if strings.Contains(result.Text, "?") {
		t.Errorf("FileStats: expected no '?' when Untracked=0, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "+2") {
		t.Errorf("FileStats: expected '+2' in Text, got %q", result.Text)
	}
}

func TestGitWidget_FileStats_DisabledByDefault(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{
		Branch:    "main",
		Modified:  5,
		Staged:    3,
		Untracked: 1,
	}}
	cfg := defaultCfg()
	// cfg.Git.FileStats is false by default

	result := Git(ctx, cfg)
	if strings.Contains(result.Text, "~") || strings.Contains(result.Text, "+") || strings.Contains(result.Text, "?") {
		t.Errorf("FileStats disabled: expected no stats in Text, got %q", result.Text)
	}
}

func TestGitWidget_FileStats_NoStatsWhenAllZero(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{
		Branch:    "main",
		Modified:  0,
		Staged:    0,
		Untracked: 0,
	}}
	cfg := defaultCfg()
	cfg.Git.FileStats = true

	result := Git(ctx, cfg)
	// Should still show branch but no stats separator+content
	if !strings.Contains(result.Text, "main") {
		t.Errorf("FileStats: expected 'main' in Text, got %q", result.Text)
	}
	// No stats means no space+stats appended beyond branch
	plain := result.PlainText
	// Plain text should just be the branch icon + "main" with no trailing stats
	if strings.Contains(plain, "~") || strings.Contains(plain, "+") || strings.Contains(plain, "?") {
		t.Errorf("FileStats all-zero: expected no stat chars in PlainText, got %q", plain)
	}
}

func TestIconsFor_Modes(t *testing.T) {
	tests := []struct {
		mode      string
		wantCheck string
	}{
		{"unicode", "✓"},
		{"ascii", "v"},
	}
	for _, tt := range tests {
		icons := IconsFor(tt.mode)
		if icons.Check != tt.wantCheck {
			t.Errorf("IconsFor(%q).Check = %q, want %q", tt.mode, icons.Check, tt.wantCheck)
		}
	}

	// Nerdfont should return non-empty
	nf := IconsFor("nerdfont")
	if nf.Check == "" {
		t.Error("nerdfont Check icon is empty")
	}

	// Unknown mode falls back to ascii
	unk := IconsFor("unknown")
	if unk.Check != "v" {
		t.Errorf("unknown mode should fall back to ascii, got Check=%q", unk.Check)
	}
}

// -- normalizeModelName -------------------------------------------------------

func TestModelWidget_DisplayNameAlreadyHasContext(t *testing.T) {
	// Claude Code sends display_name as "Opus 4.6 (1M context)".
	// normalizeModelName strips the parenthesized suffix; the model widget
	// never re-adds context size, so "context" must not appear in output.
	ctx := &model.RenderContext{
		ModelDisplayName:  "Opus 4.6 (1M context)",
		ContextWindowSize: 1000000,
	}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	if strings.Contains(got, "context") {
		t.Errorf("expected no 'context' in output, got %q", got)
	}
	if !strings.Contains(got, "Opus 4.6") {
		t.Errorf("expected 'Opus 4.6' in output, got %q", got)
	}
}

func TestNormalizeModelName_StripsParenSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Opus 4.6 (1M context)", "Opus 4.6"},
		{"Sonnet 4 (200k context)", "Sonnet 4"},
		{"Claude Haiku 4 (beta)", "Claude Haiku 4"},
		{"plain-name", "plain-name"},
	}
	for _, tt := range tests {
		got := normalizeModelName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeModelName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeModelName_BracketSuffix(t *testing.T) {
	// Model IDs with bracket annotations like "[1m]" should be stripped.
	got := normalizeModelName("claude-opus-4-6[1m]")
	want := "Claude Opus 4.6"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "claude-opus-4-6[1m]", got, want)
	}
}

func TestNormalizeModelName_4_6_Models(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-opus-4-6", "Claude Opus 4.6"},
		{"claude-sonnet-4-6", "Claude Sonnet 4.6"},
	}
	for _, tt := range tests {
		got := normalizeModelName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeModelName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeModelName_BedrockFullID(t *testing.T) {
	// Full Bedrock ID: anthropic prefix + date suffix + version suffix.
	got := normalizeModelName("anthropic.claude-sonnet-4-20250514-v1:0")
	want := "Claude Sonnet 4"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "anthropic.claude-sonnet-4-20250514-v1:0", got, want)
	}
}

func TestNormalizeModelName_DateSuffixOnly(t *testing.T) {
	got := normalizeModelName("claude-opus-4-20250601")
	want := "Claude Opus 4"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "claude-opus-4-20250601", got, want)
	}
}

func TestNormalizeModelName_VersionSuffixOnly(t *testing.T) {
	got := normalizeModelName("claude-haiku-3-5-v2:0")
	want := "Claude Haiku 3.5"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "claude-haiku-3-5-v2:0", got, want)
	}
}

func TestNormalizeModelName_AnthropicPrefixOnly(t *testing.T) {
	got := normalizeModelName("anthropic.claude-haiku-3-5")
	want := "Claude Haiku 3.5"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "anthropic.claude-haiku-3-5", got, want)
	}
}

func TestNormalizeModelName_AlreadyClean(t *testing.T) {
	// A clean slug that maps to a known display name.
	got := normalizeModelName("claude-sonnet-4")
	want := "Claude Sonnet 4"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "claude-sonnet-4", got, want)
	}
}

func TestNormalizeModelName_UnknownSlugPassthrough(t *testing.T) {
	// Unknown slugs come through as-is (after stripping prefixes/suffixes).
	got := normalizeModelName("anthropic.claude-future-9-20991231-v5:3")
	want := "claude-future-9"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "anthropic.claude-future-9-20991231-v5:3", got, want)
	}
}

func TestNormalizeModelName_PlainString(t *testing.T) {
	// A completely unrecognized string is returned unchanged.
	got := normalizeModelName("gpt-4o")
	want := "gpt-4o"
	if got != want {
		t.Errorf("normalizeModelName(%q) = %q, want %q", "gpt-4o", got, want)
	}
}

func TestModelWidget_NormalizesBedrockID(t *testing.T) {
	// The widget should display the human-readable name, not the raw Bedrock ID.
	ctx := &model.RenderContext{
		ModelDisplayName:  "anthropic.claude-sonnet-4-20250514-v1:0",
		ContextWindowSize: 200000,
	}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	if !strings.Contains(got, "Sonnet 4") {
		t.Errorf("Model widget: expected 'Sonnet 4', got %q", got)
	}
	if strings.Contains(got, "anthropic.") {
		t.Errorf("Model widget: Bedrock prefix should be stripped, got %q", got)
	}
}

// -- ModelFamilyColor via Model widget ----------------------------------------

func TestModelWidget_OpusRendersInCoral(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-opus-4-6"}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	// Coral is ANSI color 204. Verify the rendered output contains the ANSI sequence.
	coralStyle := ModelFamilyColor("Claude Opus 4.6")
	want := coralStyle.Render("Opus 4.6")
	if got != want {
		t.Errorf("Opus model: expected coral rendering %q, got %q", want, got)
	}
}

func TestModelWidget_SonnetRendersInBlue(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-sonnet-4-6"}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	blueStyle := ModelFamilyColor("Claude Sonnet 4.6")
	want := blueStyle.Render("Sonnet 4.6")
	if got != want {
		t.Errorf("Sonnet model: expected blue rendering %q, got %q", want, got)
	}
}

func TestModelWidget_HaikuRendersInGreen(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-haiku-3-5"}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	greenStyle := ModelFamilyColor("Claude Haiku 3.5")
	want := greenStyle.Render("Haiku 3.5")
	if got != want {
		t.Errorf("Haiku model: expected green rendering %q, got %q", want, got)
	}
}

func TestModelWidget_UnknownRendersInCyan(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "gpt-4o"}
	cfg := defaultCfg()

	got := Model(ctx, cfg).Text
	cyanStyle := ModelFamilyColor("gpt-4o")
	want := cyanStyle.Render("gpt-4o")
	if got != want {
		t.Errorf("Unknown model: expected cyan rendering %q, got %q", want, got)
	}
}

// -- colorStyle helper --------------------------------------------------------

func TestColorStyle_EmptyStringReturnsFallback(t *testing.T) {
	fallback := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	got := colorStyle("", fallback)
	// The returned style should equal the fallback (same ANSI output).
	text := "test"
	if got.Render(text) != fallback.Render(text) {
		t.Errorf("colorStyle(\"\", fallback) rendered differently from fallback")
	}
}

func TestColorStyle_NonEmptyStringCreatesNewStyle(t *testing.T) {
	fallback := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	override := colorStyle("1", fallback)
	// The override style must differ from the fallback (different color).
	text := "test"
	if override.Render(text) == fallback.Render(text) {
		t.Errorf("colorStyle(\"1\", fallback) should differ from fallback, both rendered %q", override.Render(text))
	}
}

// -- Context color override ---------------------------------------------------

func TestContextWidget_DefaultColorsApplied(t *testing.T) {
	// When cfg.Style.Colors fields are the defaults ("green"/"yellow"/"red"),
	// the widget must still render without error.
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "50%") {
		t.Errorf("Context with defaults: expected '50%%', got %q", got)
	}
}

func TestContextWidget_ColorOverrideApplied(t *testing.T) {
	// Setting an explicit hex color override must not break rendering.
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Style.Colors.Context = "#00ff00"
	cfg.Style.Colors.Warning = "#ffff00"
	cfg.Style.Colors.Critical = "#ff0000"

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "50%") {
		t.Errorf("Context with color overrides: expected '50%%', got %q", got)
	}
}

func TestContextWidget_EmptyColorsUseDefaults(t *testing.T) {
	// Clearing all color fields should fall back to package defaults without panicking.
	ctx := &model.RenderContext{ContextPercent: 90, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Style.Colors.Context = ""
	cfg.Style.Colors.Warning = ""
	cfg.Style.Colors.Critical = ""

	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "90%") {
		t.Errorf("Context with empty colors: expected '90%%', got %q", got)
	}
}

// -- Directory widget: style modes --------------------------------------------

func TestDirectoryWidget_StyleFull(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Style = "full"
	cfg.Directory.Levels = 3

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "Code/my-projects/tail-claude-hud") {
		t.Errorf("full style: expected last 3 segments, got %q", got)
	}
}

func TestDirectoryWidget_StyleFish(t *testing.T) {
	// Fish style abbreviates intermediate segments to first char.
	// Home dir (/Users/kyle) is replaced with ~ first, so the path becomes
	// ~/Code/my-projects/tail-claude-hud, then fish gives ~/C/m/tail-claude-hud.
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Style = "fish"

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "tail-claude-hud") {
		t.Errorf("fish style: expected last segment 'tail-claude-hud' intact, got %q", got)
	}
	// Intermediate segments must be single characters.
	if strings.Contains(got, "Code") {
		t.Errorf("fish style: 'Code' should be abbreviated to 'C', got %q", got)
	}
	if strings.Contains(got, "my-projects") {
		t.Errorf("fish style: 'my-projects' should be abbreviated to 'm', got %q", got)
	}
	if !strings.Contains(got, "~/C/m/tail-claude-hud") {
		t.Errorf("fish style: expected '~/C/m/tail-claude-hud', got %q", got)
	}
}

func TestDirectoryWidget_StyleBasename(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Style = "basename"

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "tail-claude-hud") {
		t.Errorf("basename style: expected 'tail-claude-hud', got %q", got)
	}
	if strings.Contains(got, "my-projects") {
		t.Errorf("basename style: should not contain parent segments, got %q", got)
	}
}

func TestDirectoryWidget_HomeSubstitution(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	ctx := &model.RenderContext{Cwd: home + "/Code/project"}
	cfg := defaultCfg()
	cfg.Directory.Style = "full"
	cfg.Directory.Levels = 5 // show everything

	got := Directory(ctx, cfg).Text
	// The tilde must appear; the raw home path must not.
	if !strings.Contains(got, "~") {
		t.Errorf("home substitution: expected '~' in output, got %q", got)
	}
	if strings.Contains(got, home) {
		t.Errorf("home substitution: raw home path should be replaced with '~', got %q", got)
	}
}

func TestDirectoryWidget_RootPath(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/"}
	cfg := defaultCfg()
	cfg.Directory.Style = "full"

	// Root renders as empty string from lastNSegments (no segments after split).
	got := Directory(ctx, cfg)
	// Should not panic and should produce something renderable (possibly empty text).
	_ = got
}

func TestDirectoryWidget_FishRootAbsolutePath(t *testing.T) {
	// Paths that don't start with ~ keep the leading empty segment as "/" after join.
	ctx := &model.RenderContext{Cwd: "/usr/local/bin"}
	cfg := defaultCfg()
	cfg.Directory.Style = "fish"

	got := Directory(ctx, cfg).Text
	if !strings.Contains(got, "bin") {
		t.Errorf("fish absolute: expected last segment 'bin' intact, got %q", got)
	}
	// Intermediate segments should be abbreviated.
	if strings.Contains(got, "local") {
		t.Errorf("fish absolute: 'local' should be abbreviated, got %q", got)
	}
}

func TestAbbreviateFish(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"~/Code/my-projects/tail-claude-hud", "~/C/m/tail-claude-hud"},
		{"/usr/local/bin", "/u/l/bin"},
		{"/single", "/single"},
		{"~/project", "~/project"},
		{"~/alpha/beta/gamma/deep", "~/a/b/g/deep"},
	}
	for _, tt := range tests {
		got := abbreviateFish(tt.path)
		if got != tt.want {
			t.Errorf("abbreviateFish(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// -- Configurable context thresholds ------------------------------------------

func TestContextWidget_BelowWarningUsesNormalColor(t *testing.T) {
	// At 50%, below the default warning threshold of 70%, should render in normal (green) color.
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	// Render at normal and at warning color to confirm they differ.
	got := Context(ctx, cfg).Text
	if !strings.Contains(got, "50%") {
		t.Errorf("below warning: expected '50%%' in output, got %q", got)
	}
}

func TestContextWidget_AtWarningThresholdUsesWarningColor(t *testing.T) {
	// Exactly at the warning threshold (70%) should use warning color.
	ctx := &model.RenderContext{ContextPercent: 70, ContextWindowSize: 200000}
	cfg := defaultCfg()

	normalCtx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	gotWarning := Context(ctx, cfg).Text
	gotNormal := Context(normalCtx, cfg).Text

	// The ANSI sequences must differ since colors differ.
	if gotWarning == gotNormal {
		t.Errorf("at warning threshold: expected different styling from normal, but both rendered the same ANSI output")
	}
}

func TestContextWidget_AtCriticalThresholdUsesCriticalColor(t *testing.T) {
	// Exactly at the critical threshold (85%) should use critical color.
	ctx := &model.RenderContext{ContextPercent: 85, ContextWindowSize: 200000}
	cfg := defaultCfg()

	warnCtx := &model.RenderContext{ContextPercent: 70, ContextWindowSize: 200000}
	gotCritical := Context(ctx, cfg).Text
	gotWarning := Context(warnCtx, cfg).Text

	if gotCritical == gotWarning {
		t.Errorf("at critical threshold: expected different styling from warning, but both rendered the same ANSI output")
	}
}

func TestContextWidget_AboveCriticalThresholdUsesCriticalColor(t *testing.T) {
	// Above critical (95%) should use the same critical color as at 85%.
	// Disable breakdown so both render as a simple percent label with no suffix.
	// Use ascii icons so circle-slice icons don't vary between percent values.
	ctx95 := &model.RenderContext{ContextPercent: 95, ContextWindowSize: 200000}
	ctx85 := &model.RenderContext{ContextPercent: 85, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.ShowBreakdown = false
	cfg.Style.Icons = "ascii"

	got95 := Context(ctx95, cfg).Text
	got85 := Context(ctx85, cfg).Text

	if !strings.Contains(got95, "95%") {
		t.Errorf("above critical: expected '95%%' in output, got %q", got95)
	}
	// Both should use the same critical style — strip the percent digits and compare.
	ansi95 := strings.Replace(got95, "95", "XX", 1)
	ansi85 := strings.Replace(got85, "85", "XX", 1)
	if ansi95 != ansi85 {
		t.Errorf("above critical: expected same ANSI structure as at-critical, got %q vs %q", ansi95, ansi85)
	}
}

func TestContextWidget_CustomWarningThreshold(t *testing.T) {
	// A custom warning threshold of 60 should trigger warning color at 60%.
	ctx60 := &model.RenderContext{ContextPercent: 60, ContextWindowSize: 200000}
	ctx50 := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Thresholds.ContextWarning = 60
	cfg.Thresholds.ContextCritical = 80

	got60 := Context(ctx60, cfg).Text
	got50 := Context(ctx50, cfg).Text

	// 60% should use warning color; 50% should use normal color.
	if got60 == got50 {
		t.Errorf("custom warning=60: expected different styling at 60%% vs 50%%")
	}
}

// -- Cost widget --------------------------------------------------------------

func TestCostWidget_ZeroReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{SessionCostUSD: 0}
	cfg := defaultCfg()

	if got := Cost(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Cost zero: expected empty, got %q", got.Text)
	}
}

func TestCostWidget_BelowWarningUsesNormalColor(t *testing.T) {
	ctx := &model.RenderContext{SessionCostUSD: 2.50}
	cfg := defaultCfg()

	got := Cost(ctx, cfg)
	if !strings.Contains(got.Text, "$2.50") {
		t.Errorf("Cost below warning: expected '$2.50' in output, got %q", got.Text)
	}
}

func TestCostWidget_AtWarningThresholdUsesWarningColor(t *testing.T) {
	ctxWarn := &model.RenderContext{SessionCostUSD: 5.00}
	ctxNorm := &model.RenderContext{SessionCostUSD: 2.00}
	cfg := defaultCfg()

	gotWarn := Cost(ctxWarn, cfg)
	gotNorm := Cost(ctxNorm, cfg)

	if gotWarn.Text == gotNorm.Text {
		t.Errorf("Cost at warning: expected different styling from normal")
	}
	if !strings.Contains(gotWarn.Text, "$5.00") {
		t.Errorf("Cost at warning: expected '$5.00' in output, got %q", gotWarn.Text)
	}
}

func TestCostWidget_AtCriticalThresholdUsesCriticalColor(t *testing.T) {
	ctxCrit := &model.RenderContext{SessionCostUSD: 10.00}
	ctxWarn := &model.RenderContext{SessionCostUSD: 5.00}
	cfg := defaultCfg()

	gotCrit := Cost(ctxCrit, cfg)
	gotWarn := Cost(ctxWarn, cfg)

	if gotCrit.Text == gotWarn.Text {
		t.Errorf("Cost at critical: expected different styling from warning")
	}
	if !strings.Contains(gotCrit.Text, "$10.00") {
		t.Errorf("Cost at critical: expected '$10.00' in output, got %q", gotCrit.Text)
	}
}

func TestCostWidget_AboveCriticalUsesCriticalColor(t *testing.T) {
	ctx15 := &model.RenderContext{SessionCostUSD: 15.00}
	ctx10 := &model.RenderContext{SessionCostUSD: 10.00}
	cfg := defaultCfg()

	got15 := Cost(ctx15, cfg)
	got10 := Cost(ctx10, cfg)

	// Both should use critical color; strip dollar amounts and compare ANSI structure.
	ansi15 := strings.Replace(got15.Text, "15.00", "XX.XX", 1)
	ansi10 := strings.Replace(got10.Text, "10.00", "XX.XX", 1)
	if ansi15 != ansi10 {
		t.Errorf("Cost above critical: expected same ANSI structure as at-critical, got %q vs %q", ansi15, ansi10)
	}
}

func TestCostWidget_CustomThresholds(t *testing.T) {
	// Custom thresholds: warn at $2, critical at $4.
	cfg := defaultCfg()
	cfg.Thresholds.CostWarning = 2.00
	cfg.Thresholds.CostCritical = 4.00

	ctxNorm := &model.RenderContext{SessionCostUSD: 1.00}
	ctxWarn := &model.RenderContext{SessionCostUSD: 2.00}
	ctxCrit := &model.RenderContext{SessionCostUSD: 4.00}

	gotNorm := Cost(ctxNorm, cfg)
	gotWarn := Cost(ctxWarn, cfg)
	gotCrit := Cost(ctxCrit, cfg)

	if gotNorm.Text == gotWarn.Text {
		t.Errorf("custom thresholds: normal and warning should differ")
	}
	if gotWarn.Text == gotCrit.Text {
		t.Errorf("custom thresholds: warning and critical should differ")
	}
	if !strings.Contains(gotCrit.Text, "$4.00") {
		t.Errorf("custom critical: expected '$4.00', got %q", gotCrit.Text)
	}
}

func TestLastNSegments(t *testing.T) {
	tests := []struct {
		path string
		n    int
		want string
	}{
		{"/Users/kyle/Code", 1, "Code"},
		{"/Users/kyle/Code", 2, "kyle/Code"},
		{"/Users/kyle/Code", 5, "Users/kyle/Code"},
		{"relative/path", 1, "path"},
		{"/trailing/slash/", 1, "slash"},
		{"", 1, ""},
		{"/", 1, ""},
	}

	for _, tt := range tests {
		got := lastNSegments(tt.path, tt.n)
		if got != tt.want {
			t.Errorf("lastNSegments(%q, %d) = %q, want %q", tt.path, tt.n, got, tt.want)
		}
	}
}

// -- Session widget -----------------------------------------------------------

func TestSessionWidget_NilTranscriptReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: nil}
	cfg := defaultCfg()

	if got := Session(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Session with nil Transcript: expected empty, got %q", got.Text)
	}
}

func TestSessionWidget_EmptySessionNameReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{SessionName: ""}}
	cfg := defaultCfg()

	if got := Session(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Session with empty SessionName: expected empty, got %q", got.Text)
	}
}

func TestSessionWidget_RendersSessionName(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{SessionName: "my-feature-branch"}}
	cfg := defaultCfg()

	got := Session(ctx, cfg).Text
	if !strings.Contains(got, "my-feature-branch") {
		t.Errorf("Session: expected 'my-feature-branch' in output, got %q", got)
	}
}

func TestSessionWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["session"]; !ok {
		t.Error("Registry missing 'session' widget")
	}
}

// -- New icon fields ----------------------------------------------------------

func TestIconsFor_NewFieldsNonEmpty(t *testing.T) {
	modes := []string{"nerdfont", "unicode", "ascii"}
	for _, mode := range modes {
		icons := IconsFor(mode)
		fields := map[string]string{
			"Read":     icons.Read,
			"Edit":     icons.Edit,
			"Write":    icons.Write,
			"Bash":     icons.Bash,
			"Grep":     icons.Grep,
			"Glob":     icons.Glob,
			"Web":      icons.Web,
			"Task":     icons.Task,
			"Skill":    icons.Skill,
			"Thinking": icons.Thinking,
			"Other":    icons.Other,
			"Error":    icons.Error,
		}
		for name, val := range fields {
			if val == "" {
				t.Errorf("IconsFor(%q).%s is empty", mode, name)
			}
		}
	}
}

// -- CategoryIcon -------------------------------------------------------------

func TestCategoryIcon_KnownCategories(t *testing.T) {
	icons := IconsFor("ascii")
	tests := []struct {
		category string
		want     string
	}{
		{"Read", icons.Read},
		{"Edit", icons.Edit},
		{"Write", icons.Write},
		{"Bash", icons.Bash},
		{"Grep", icons.Grep},
		{"Glob", icons.Glob},
		{"Web", icons.Web},
		{"Task", icons.Task},
		{"Skill", icons.Skill},
		{"Thinking", icons.Thinking},
	}
	for _, tt := range tests {
		got := CategoryIcon(icons, tt.category)
		if got != tt.want {
			t.Errorf("CategoryIcon(ascii, %q) = %q, want %q", tt.category, got, tt.want)
		}
	}
}

func TestCategoryIcon_UnknownFallsBackToOther(t *testing.T) {
	icons := IconsFor("ascii")
	got := CategoryIcon(icons, "unknown-category")
	if got != icons.Other {
		t.Errorf("CategoryIcon(ascii, \"unknown-category\") = %q, want Other %q", got, icons.Other)
	}
}

func TestCategoryIcon_AllModesReturnNonEmpty(t *testing.T) {
	categories := []string{"Read", "Edit", "Write", "Bash", "Grep", "Glob", "Web", "Task", "Skill", "Thinking", "unknown"}
	modes := []string{"nerdfont", "unicode", "ascii"}
	for _, mode := range modes {
		icons := IconsFor(mode)
		for _, cat := range categories {
			got := CategoryIcon(icons, cat)
			if got == "" {
				t.Errorf("CategoryIcon(%q, %q) returned empty string", mode, cat)
			}
		}
	}
}

// -- Running icon field -------------------------------------------------------

// Verify that IconsFor exposes a non-empty Running field across all icon modes.
func TestIconsFor_RunningFieldNonEmpty(t *testing.T) {
	for _, mode := range []string{"nerdfont", "unicode", "ascii"} {
		icons := IconsFor(mode)
		if icons.Running == "" {
			t.Errorf("IconsFor(%q).Running is empty", mode)
		}
	}
}

// -- Duration widget: stdin TotalDurationMs priority -------------------------

// When TotalDurationMs is set, the duration widget must use it rather than
// deriving elapsed time from SessionStart. This ensures the authoritative
// Claude Code duration takes precedence over the transcript-derived estimate.
func TestDurationWidget_PrefersStdinDuration(t *testing.T) {
	// TotalDurationMs = 3 minutes 5 seconds (185 000 ms).
	// SessionStart is set to a very old time so that transcript-derived duration
	// would yield a completely different result, confirming we used TotalDurationMs.
	ctx := &model.RenderContext{
		TotalDurationMs: 185_000,
		SessionStart:    time.Now().Add(-10 * time.Hour).UTC().Format(time.RFC3339),
	}
	cfg := defaultCfg()

	got := Duration(ctx, cfg).Text
	// 185 000 ms rounds to 3m 5s — check both components.
	if !strings.Contains(got, "3m") {
		t.Errorf("Duration with TotalDurationMs=185000: expected '3m', got %q", got)
	}
	if !strings.Contains(got, "5s") {
		t.Errorf("Duration with TotalDurationMs=185000: expected '5s', got %q", got)
	}
	// Confirm it did NOT derive from the 10-hour SessionStart (would be "10h ...").
	if strings.Contains(got, "10h") {
		t.Errorf("Duration with TotalDurationMs set: must not fall back to SessionStart, got %q", got)
	}
}

// When TotalDurationMs is zero, the widget must fall back to SessionStart.
func TestDurationWidget_FallsBackToSessionStartWhenNoDuration(t *testing.T) {
	ctx := &model.RenderContext{
		TotalDurationMs: 0,
		SessionStart:    time.Now().Add(-5*time.Minute - 30*time.Second).UTC().Format(time.RFC3339),
	}
	cfg := defaultCfg()

	got := Duration(ctx, cfg).Text
	// Should show minutes (≈5m).
	if !strings.Contains(got, "m") {
		t.Errorf("Duration fallback to SessionStart: expected 'm' in output, got %q", got)
	}
}

// When both TotalDurationMs and SessionStart are absent, the widget returns empty.
func TestDurationWidget_NeitherSourceReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{
		TotalDurationMs: 0,
		SessionStart:    "",
	}
	cfg := defaultCfg()

	if got := Duration(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Duration with no data: expected empty, got %q", got.Text)
	}
}

// -- WidgetResult -------------------------------------------------------------

// TestWidgetResult_IsEmpty verifies the IsEmpty helper.
func TestWidgetResult_IsEmpty(t *testing.T) {
	if !(WidgetResult{}).IsEmpty() {
		t.Error("zero-value WidgetResult should be empty")
	}
	if (WidgetResult{Text: "x"}).IsEmpty() {
		t.Error("WidgetResult with Text should not be empty")
	}
	// FgColor alone does not make a result non-empty.
	if !(WidgetResult{FgColor: "42"}).IsEmpty() {
		t.Error("WidgetResult with only FgColor should still be empty")
	}
}

// TestWidgetResult_DualOutput verifies that the env widget sets both Text
// (pre-styled for plain mode) and PlainText+FgColor (for powerline/minimal).
func TestWidgetResult_DualOutput(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 3, Hooks: 2}}
	cfg := defaultCfg()

	result := Env(ctx, cfg)

	// FgColor must be "8" (MutedStyle's color) for powerline/minimal rendering.
	if result.FgColor != "8" {
		t.Errorf("Env FgColor: expected '8', got %q", result.FgColor)
	}

	// PlainText must be the unstyled content.
	if result.PlainText != "3M 2H" {
		t.Errorf("Env PlainText: expected '3M 2H', got %q", result.PlainText)
	}

	// Text must be the pre-styled version for plain mode.
	want := envStyle.Render("3M 2H")
	if result.Text != want {
		t.Errorf("Env Text: expected pre-styled %q, got %q", want, result.Text)
	}
}

// -- percentToIcon ------------------------------------------------------------

// TestPercentToIcon_BoundaryValues checks that each of the 8 icon levels is
// selected at the correct boundary and that edge cases (0, 100, negative) clamp
// to the first or last icon.
func TestPercentToIcon_BoundaryValues(t *testing.T) {
	// The 8 Nerd Font circle-slice icons in codepoint order.
	icons := [8]string{
		"\U000F0A9E", // level 1 (nearly empty)
		"\U000F0A9F", // level 2
		"\U000F0AA0", // level 3
		"\U000F0AA1", // level 4
		"\U000F0AA2", // level 5
		"\U000F0AA3", // level 6
		"\U000F0AA4", // level 7
		"\U000F0AA5", // level 8 (fully filled)
	}

	tests := []struct {
		percent int
		want    string
		label   string
	}{
		// Clamping edges
		{-1, icons[0], "negative clamps to first"},
		{0, icons[0], "0% is first icon"},
		{100, icons[7], "100% is last icon"},
		{101, icons[7], "over 100 clamps to last"},

		// Nearest-icon mapping: round(percent / 12.5) - 1
		// idx 0 (1/8 = 12.5%): 0–18%
		{1, icons[0], "1% → idx 0"},
		{18, icons[0], "18% → idx 0"},
		// idx 1 (2/8 = 25%): 19–31%
		{19, icons[1], "19% → idx 1"},
		{31, icons[1], "31% → idx 1"},
		// idx 2 (3/8 = 37.5%): 32–43%
		{32, icons[2], "32% → idx 2"},
		{43, icons[2], "43% → idx 2"},
		// idx 3 (4/8 = 50%): 44–56%
		{44, icons[3], "44% → idx 3"},
		{50, icons[3], "50% → idx 3"},
		{56, icons[3], "56% → idx 3"},
		// idx 4 (5/8 = 62.5%): 57–68%
		{57, icons[4], "57% → idx 4"},
		{68, icons[4], "68% → idx 4"},
		// idx 5 (6/8 = 75%): 69–81%
		{69, icons[5], "69% → idx 5"},
		{81, icons[5], "81% → idx 5"},
		// idx 6 (7/8 = 87.5%): 82–93%
		{82, icons[6], "82% → idx 6"},
		{93, icons[6], "93% → idx 6"},
		// idx 7 (8/8 = 100%): 94–99%
		{94, icons[7], "94% → idx 7"},
		{99, icons[7], "99% → idx 7"},
	}

	for _, tt := range tests {
		got := percentToIcon(tt.percent)
		if got != tt.want {
			t.Errorf("percentToIcon(%d) [%s]: got %q, want %q", tt.percent, tt.label, got, tt.want)
		}
	}
}

// TestContextWidget_NerdfontIconPrepended verifies that a circle-slice icon is prepended
// to the context output when cfg.Style.Icons is "nerdfont".
func TestContextWidget_NerdfontIconPrepended(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Style.Icons = "nerdfont"

	got := Context(ctx, cfg)
	// 50% maps to index 3 (44-56% range) → circle_slice_4 (U+F0AA1).
	wantIcon := "\U000F0AA1"
	if !strings.Contains(got.Text, wantIcon) {
		t.Errorf("Context(nerdfont, 50%%): expected circle-slice icon %q in output, got %q", wantIcon, got.Text)
	}
}

// TestContextWidget_NoIconWithoutNerdfont confirms the icon is absent in non-nerdfont modes.
func TestContextWidget_NoIconWithoutNerdfont(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Context(ctx, cfg)
	// None of the circle-slice icons should appear.
	for _, icon := range []string{"\U000F0A9E", "\U000F0A9F", "\U000F0AA0", "\U000F0AA1",
		"\U000F0AA2", "\U000F0AA3", "\U000F0AA4", "\U000F0AA5"} {
		if strings.Contains(got.Text, icon) {
			t.Errorf("Context(ascii): unexpected circle-slice icon in output %q", got.Text)
		}
	}
}
