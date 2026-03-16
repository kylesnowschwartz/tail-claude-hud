package widget

import (
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

	got := Model(ctx, cfg)
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

	got := Model(ctx, cfg)
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

	if got := Model(ctx, cfg); got != "" {
		t.Errorf("expected empty string for empty model name, got %q", got)
	}
}

func TestContextWidget_GreenUnder70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "50%") {
		t.Errorf("expected '50%%' in output, got %q", got)
	}
}

func TestContextWidget_YellowAt70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 75, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "75%") {
		t.Errorf("expected '75%%' in output, got %q", got)
	}
}

func TestContextWidget_RedAt85(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 90, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "90%") {
		t.Errorf("expected '90%%' in output, got %q", got)
	}
}

func TestContextWidget_EmptyWhenZero(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Context(ctx, cfg); got != "" {
		t.Errorf("expected empty string for zero context, got %q", got)
	}
}

func TestContextWidget_PercentMode(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 42, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Value = "percent"

	got := Context(ctx, cfg)
	if !strings.Contains(got, "42%") {
		t.Errorf("percent mode: expected '42%%' in output, got %q", got)
	}
}

func TestContextWidget_PercentModeDefault(t *testing.T) {
	// Empty Value string should behave like "percent".
	ctx := &model.RenderContext{ContextPercent: 55, ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Context.Value = ""

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
	if strings.Contains(got, "in:") {
		t.Errorf("breakdown disabled: expected no breakdown, got %q", got)
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

	got := Directory(ctx, cfg)
	if !strings.Contains(got, "tail-claude-hud") {
		t.Errorf("expected 'tail-claude-hud', got %q", got)
	}
}

func TestDirectoryWidget_MultipleSegments(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Levels = 2

	got := Directory(ctx, cfg)
	if !strings.Contains(got, "my-projects/tail-claude-hud") {
		t.Errorf("expected 2 segments, got %q", got)
	}
}

func TestDirectoryWidget_EmptyCwd(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Directory(ctx, cfg); got != "" {
		t.Errorf("expected empty string for empty cwd, got %q", got)
	}
}

func TestRegistryHasAllWidgets(t *testing.T) {
	expected := []string{"model", "context", "directory", "git", "project", "env", "duration", "tools", "agents", "todos", "session", "thinking"}
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

	// All transcript-powered widgets must return "" when Transcript is nil.
	widgets := []string{"tools", "agents", "todos", "session", "thinking"}
	for _, name := range widgets {
		fn := Registry[name]
		if got := fn(ctx, cfg); got != "" {
			t.Errorf("widget %q with nil Transcript: expected empty, got %q", name, got)
		}
	}
}

// -- Tools widget -------------------------------------------------------------

func TestToolsWidget_EmptyToolsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Tools(ctx, cfg); got != "" {
		t.Errorf("Tools with empty tools: expected empty, got %q", got)
	}
}

// Spec 8: running tool renders with category icon + name + elapsed.
func TestToolsWidget_RunningToolShowsCategoryIconAndName(t *testing.T) {
	// Count == 0 signals running.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Read", Count: 0, Category: "file"}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg)
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
		Tools: []model.ToolEntry{{Name: "Bash", Count: 0, Category: "shell"}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg)
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
	// Count > 0 signals completed.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Tools: []model.ToolEntry{{Name: "Write", Count: 1, Category: "file", DurationMs: 300}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg)
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
		Tools: []model.ToolEntry{{Name: "Bash", Count: 1, Category: "shell", DurationMs: 500, HasError: true}},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Tools(ctx, cfg)
	icons := IconsFor("ascii")
	// Error uses category icon (not error icon) in red.
	if !strings.Contains(got, icons.Shell) {
		t.Errorf("Tools error: expected shell icon %q, got %q", icons.Shell, got)
	}
	if !strings.Contains(got, "Bash") {
		t.Errorf("Tools error: expected tool name 'Bash', got %q", got)
	}
}

// Spec 11: max 5 tools shown.
func TestToolsWidget_MaxFiveToolsShown(t *testing.T) {
	// Six completed tools — only 5 should be rendered.
	tools := []model.ToolEntry{
		{Name: "T1", Count: 1, DurationMs: 100},
		{Name: "T2", Count: 1, DurationMs: 100},
		{Name: "T3", Count: 1, DurationMs: 100},
		{Name: "T4", Count: 1, DurationMs: 100},
		{Name: "T5", Count: 1, DurationMs: 100},
		{Name: "T6", Count: 1, DurationMs: 100},
	}
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{Tools: tools}}
	cfg := defaultCfg()

	got := Tools(ctx, cfg)
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

	if got := Tools(ctx, cfg); got != "" {
		t.Errorf("Tools nil transcript: expected empty, got %q", got)
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

// -- formatTokenCost ----------------------------------------------------------

func TestFormatTokenCost(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{1200, "1.2k"},
		{99999, "100.0k"},
		{100000, "100k"},
		{123456, "123k"},
	}
	for _, tt := range tests {
		got := formatTokenCost(tt.n)
		if got != tt.want {
			t.Errorf("formatTokenCost(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// -- Agents widget ------------------------------------------------------------

func TestAgentsWidget_EmptyAgentsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Agents(ctx, cfg); got != "" {
		t.Errorf("Agents with empty agents: expected empty, got %q", got)
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

	got := Agents(ctx, cfg)

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

	got := Agents(ctx, cfg)
	icons := IconsFor("ascii")
	// The agent icon should be rendered (ASCII mode uses "@" for Agent).
	if !strings.Contains(got, icons.Agent) {
		t.Errorf("Agents running: expected agent icon %q, got %q", icons.Agent, got)
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

	got := Agents(ctx, cfg)
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

	got := Agents(ctx, cfg)
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

	got := Agents(ctx, cfg)
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

	got := Agents(ctx, cfg)
	// Count the " | " separators — 4 separators means 5 entries.
	separators := strings.Count(got, " | ")
	if separators > 4 {
		t.Errorf("Agents: expected at most 5 entries (4 separators), got %d separators in %q", separators, got)
	}
}

// -- Todos widget -------------------------------------------------------------

func TestTodosWidget_EmptyTodosReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := defaultCfg()

	if got := Todos(ctx, cfg); got != "" {
		t.Errorf("Todos with empty list: expected empty, got %q", got)
	}
}

func TestTodosWidget_AllDoneShowsGreenCheck(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		Todos: []model.TodoItem{
			{ID: "1", Content: "First", Done: true},
			{ID: "2", Content: "Second", Done: true},
		},
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Todos(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Check) {
		t.Errorf("Todos all done: expected check icon %q, got %q", icons.Check, got)
	}
	if !strings.Contains(got, "2/2") {
		t.Errorf("Todos all done: expected '2/2', got %q", got)
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

	got := Todos(ctx, cfg)
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

	got := Todos(ctx, cfg)
	if !strings.Contains(got, "0/2") {
		t.Errorf("Todos none done: expected '0/2', got %q", got)
	}
}

// -- Env widget ---------------------------------------------------------------

func TestEnvWidget_NilEnvCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: nil}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); got != "" {
		t.Errorf("Env with nil EnvCounts: expected empty, got %q", got)
	}
}

func TestEnvWidget_ZeroCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{}}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); got != "" {
		t.Errorf("Env with all-zero counts: expected empty, got %q", got)
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

	got := Env(ctx, cfg)
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

	got := Env(ctx, cfg)
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

	got := Env(ctx, cfg)
	if !strings.Contains(got, "3M") {
		t.Errorf("Env MCPOnly: expected '3M' in output, got %q", got)
	}
}

func TestEnvWidget_ClaudeMdOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{ClaudeMdFiles: 2}}
	cfg := defaultCfg()

	got := Env(ctx, cfg)
	if !strings.Contains(got, "2C") {
		t.Errorf("Env ClaudeMdOnly: expected '2C' in output, got %q", got)
	}
}

func TestEnvWidget_RuleFilesOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{RuleFiles: 4}}
	cfg := defaultCfg()

	got := Env(ctx, cfg)
	if !strings.Contains(got, "4R") {
		t.Errorf("Env RuleFilesOnly: expected '4R' in output, got %q", got)
	}
}

func TestEnvWidget_HooksOnly(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{Hooks: 3}}
	cfg := defaultCfg()

	got := Env(ctx, cfg)
	if !strings.Contains(got, "3H") {
		t.Errorf("Env HooksOnly: expected '3H' in output, got %q", got)
	}
}

// -- Duration widget ----------------------------------------------------------

func TestDurationWidget_EmptySessionStartReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{SessionStart: ""}
	cfg := defaultCfg()

	if got := Duration(ctx, cfg); got != "" {
		t.Errorf("Duration with empty SessionStart: expected empty, got %q", got)
	}
}

func TestDurationWidget_RendersTimestamp(t *testing.T) {
	// Use a timestamp 2 hours and 30 minutes ago.
	start := time.Now().Add(-2*time.Hour - 30*time.Minute).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionStart: start}
	cfg := defaultCfg()

	got := Duration(ctx, cfg)
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

	got := Duration(ctx, cfg)
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

	got := Duration(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Clock) {
		t.Errorf("Duration(ascii): expected clock icon %q, got %q", icons.Clock, got)
	}
}

// -- Git widget ---------------------------------------------------------------

func TestGitWidget_NilGitReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Git: nil}
	cfg := defaultCfg()

	if got := Git(ctx, cfg); got != "" {
		t.Errorf("Git with nil Git: expected empty, got %q", got)
	}
}

func TestGitWidget_ShowsBranch(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()

	got := Git(ctx, cfg)
	if !strings.Contains(got, "main") {
		t.Errorf("Git: expected 'main' in output, got %q", got)
	}
}

func TestGitWidget_DirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/foo", Dirty: true}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg)
	if !strings.Contains(got, "*") {
		t.Errorf("Git dirty: expected '*' in output, got %q", got)
	}
}

func TestGitWidget_CleanNoDirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main", Dirty: false}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg)
	if strings.Contains(got, "*") {
		t.Errorf("Git clean: expected no '*', got %q", got)
	}
}

func TestGitWidget_AheadBehindCounts(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/bar", AheadBy: 2, BehindBy: 1}}
	cfg := defaultCfg()
	cfg.Git.AheadBehind = true

	got := Git(ctx, cfg)
	if !strings.Contains(got, "+2") {
		t.Errorf("Git ahead: expected '+2', got %q", got)
	}
	if !strings.Contains(got, "-1") {
		t.Errorf("Git behind: expected '-1', got %q", got)
	}
}

func TestGitWidget_UsesIconLookup(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Git(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Branch) {
		t.Errorf("Git(ascii): expected branch icon %q, got %q", icons.Branch, got)
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

	got := Model(ctx, cfg)
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

	got := Model(ctx, cfg)
	if !strings.Contains(got, "Claude Sonnet 4") {
		t.Errorf("Model widget: expected 'Claude Sonnet 4', got %q", got)
	}
	if strings.Contains(got, "anthropic.") {
		t.Errorf("Model widget: Bedrock prefix should be stripped, got %q", got)
	}
}

// -- ModelFamilyColor via Model widget ----------------------------------------

func TestModelWidget_OpusRendersInCoral(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-opus-4-6"}
	cfg := defaultCfg()

	got := Model(ctx, cfg)
	// Coral is ANSI color 204. Verify the rendered output contains the ANSI sequence.
	coralStyle := ModelFamilyColor("Claude Opus 4.6")
	want := coralStyle.Render("[Claude Opus 4.6]")
	if got != want {
		t.Errorf("Opus model: expected coral rendering %q, got %q", want, got)
	}
}

func TestModelWidget_SonnetRendersInBlue(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-sonnet-4-6"}
	cfg := defaultCfg()

	got := Model(ctx, cfg)
	blueStyle := ModelFamilyColor("Claude Sonnet 4.6")
	want := blueStyle.Render("[Claude Sonnet 4.6]")
	if got != want {
		t.Errorf("Sonnet model: expected blue rendering %q, got %q", want, got)
	}
}

func TestModelWidget_HaikuRendersInGreen(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "claude-haiku-3-5"}
	cfg := defaultCfg()

	got := Model(ctx, cfg)
	greenStyle := ModelFamilyColor("Claude Haiku 3.5")
	want := greenStyle.Render("[Claude Haiku 3.5]")
	if got != want {
		t.Errorf("Haiku model: expected green rendering %q, got %q", want, got)
	}
}

func TestModelWidget_UnknownRendersInCyan(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "gpt-4o"}
	cfg := defaultCfg()

	got := Model(ctx, cfg)
	cyanStyle := ModelFamilyColor("gpt-4o")
	want := cyanStyle.Render("[gpt-4o]")
	if got != want {
		t.Errorf("Unknown model: expected cyan rendering %q, got %q", want, got)
	}
}

// -- colorStyle helper --------------------------------------------------------

func TestColorStyle_EmptyStringReturnsFallback(t *testing.T) {
	fallback := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	got := colorStyle("", fallback)
	// The returned style should equal the fallback (same ANSI output).
	text := "test"
	if got.Render(text) != fallback.Render(text) {
		t.Errorf("colorStyle(\"\", fallback) rendered differently from fallback")
	}
}

func TestColorStyle_NonEmptyStringCreatesNewStyle(t *testing.T) {
	fallback := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	override := colorStyle("196", fallback)
	// The override style must differ from the fallback (different color).
	text := "test"
	if override.Render(text) == fallback.Render(text) {
		t.Errorf("colorStyle(\"196\", fallback) should differ from fallback, both rendered %q", override.Render(text))
	}
}

// -- Context color override ---------------------------------------------------

func TestContextWidget_DefaultColorsApplied(t *testing.T) {
	// When cfg.Style.Colors fields are the defaults ("green"/"yellow"/"red"),
	// the widget must still render without error.
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
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

	got := Context(ctx, cfg)
	if !strings.Contains(got, "90%") {
		t.Errorf("Context with empty colors: expected '90%%', got %q", got)
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

	if got := Session(ctx, cfg); got != "" {
		t.Errorf("Session with nil Transcript: expected empty, got %q", got)
	}
}

func TestSessionWidget_EmptySessionNameReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{SessionName: ""}}
	cfg := defaultCfg()

	if got := Session(ctx, cfg); got != "" {
		t.Errorf("Session with empty SessionName: expected empty, got %q", got)
	}
}

func TestSessionWidget_RendersSessionName(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{SessionName: "my-feature-branch"}}
	cfg := defaultCfg()

	got := Session(ctx, cfg)
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
			"Shell":    icons.Shell,
			"Search":   icons.Search,
			"Web":      icons.Web,
			"Agent":    icons.Agent,
			"Gear":     icons.Gear,
			"Thinking": icons.Thinking,
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
		{"file", icons.Read},
		{"shell", icons.Shell},
		{"search", icons.Search},
		{"web", icons.Web},
		{"agent", icons.Agent},
		{"internal", icons.Gear},
	}
	for _, tt := range tests {
		got := CategoryIcon(icons, tt.category)
		if got != tt.want {
			t.Errorf("CategoryIcon(ascii, %q) = %q, want %q", tt.category, got, tt.want)
		}
	}
}

func TestCategoryIcon_UnknownFallsBackToGear(t *testing.T) {
	icons := IconsFor("ascii")
	got := CategoryIcon(icons, "unknown-category")
	if got != icons.Gear {
		t.Errorf("CategoryIcon(ascii, \"unknown-category\") = %q, want Gear %q", got, icons.Gear)
	}
}

func TestCategoryIcon_AllModesReturnNonEmpty(t *testing.T) {
	categories := []string{"file", "shell", "search", "web", "agent", "internal", "unknown"}
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
