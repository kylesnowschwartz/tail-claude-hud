package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func longAgent(name string, status string) model.AgentEntry {
	return model.AgentEntry{
		Name:       name,
		Status:     status,
		StartTime:  time.Now().Add(-30 * time.Second),
		DurationMs: 5000,
		ColorIndex: 0,
	}
}

func TestAgents_ThreeLongDescriptions_Width120_TruncatedNames(t *testing.T) {
	agents := []model.AgentEntry{
		longAgent("Implement comprehensive test coverage for parser", "running"),
		longAgent("Refactor authentication middleware for OAuth2 flow", "running"),
		longAgent("Review structural completeness of API endpoints", "running"),
	}
	ctx := &model.RenderContext{
		TerminalWidth: 120,
		Transcript:    &model.TranscriptData{Agents: agents},
	}
	cfg := defaultCfg()

	result := Agents(ctx, cfg)
	if result.IsEmpty() {
		t.Fatal("expected non-empty result for 3 agents at width 120")
	}

	// Per-entry name truncation should have kicked in — original names are
	// >25 chars and should not appear verbatim.
	for _, a := range agents {
		if strings.Contains(result.PlainText, a.Name) {
			t.Errorf("expected agent name %q to be truncated, but found it verbatim", a.Name)
		}
	}

	// At least some entries should show the truncation ellipsis.
	if !strings.Contains(result.PlainText, "…") {
		t.Errorf("expected truncation ellipsis '…' in output, got %q", result.PlainText)
	}

	// At width 120, at least 2 agents should be visible (with or without "+N more").
	// Each truncated entry is ~40 chars, so 2 fit comfortably.
	entryCount := strings.Count(result.PlainText, "…")
	if entryCount < 2 {
		t.Errorf("expected at least 2 truncated agent entries at width 120, got %d in %q", entryCount, result.PlainText)
	}
}

func TestAgents_ThreeLongDescriptions_Width80_TwoVisiblePlusMore(t *testing.T) {
	agents := []model.AgentEntry{
		longAgent("Implement comprehensive test coverage for parser", "running"),
		longAgent("Refactor authentication middleware for OAuth2 flow", "running"),
		longAgent("Review structural completeness of API endpoints", "running"),
	}
	ctx := &model.RenderContext{
		TerminalWidth: 80,
		Transcript:    &model.TranscriptData{Agents: agents},
	}
	cfg := defaultCfg()

	result := Agents(ctx, cfg)
	if result.IsEmpty() {
		t.Fatal("expected non-empty result for 3 agents at width 80")
	}

	// At width 80, not all 3 should fit — expect a "+N more" indicator.
	if !strings.Contains(result.PlainText, "more") {
		t.Errorf("expected '+N more' at width 80, got %q", result.PlainText)
	}
}

func TestAgents_OneLongDescription_Truncated_NoMore(t *testing.T) {
	agents := []model.AgentEntry{
		longAgent("Implement comprehensive test coverage for the entire parser subsystem", "running"),
	}
	ctx := &model.RenderContext{
		TerminalWidth: 120,
		Transcript:    &model.TranscriptData{Agents: agents},
	}
	cfg := defaultCfg()

	result := Agents(ctx, cfg)
	if result.IsEmpty() {
		t.Fatal("expected non-empty result for 1 agent")
	}

	// Name should be truncated.
	if strings.Contains(result.PlainText, agents[0].Name) {
		t.Errorf("expected long name to be truncated, but found verbatim: %q", result.PlainText)
	}

	// No "+N more" since there's only one agent.
	if strings.Contains(result.PlainText, "more") {
		t.Errorf("expected no '+N more' for single agent, got %q", result.PlainText)
	}
}

func TestAgents_ShortNames_NoTruncation(t *testing.T) {
	agents := []model.AgentEntry{
		longAgent("Explore", "running"),
		longAgent("Plan", "running"),
	}
	ctx := &model.RenderContext{
		TerminalWidth: 120,
		Transcript:    &model.TranscriptData{Agents: agents},
	}
	cfg := defaultCfg()

	result := Agents(ctx, cfg)
	if result.IsEmpty() {
		t.Fatal("expected non-empty result")
	}

	// Short names should appear verbatim — no truncation ellipsis.
	if !strings.Contains(result.PlainText, "Explore") {
		t.Errorf("expected 'Explore' verbatim in output, got %q", result.PlainText)
	}
	if !strings.Contains(result.PlainText, "Plan") {
		t.Errorf("expected 'Plan' verbatim in output, got %q", result.PlainText)
	}
}

func TestTruncateAgentName_Short(t *testing.T) {
	name := "Explore"
	got := truncateAgentName(name, maxAgentNameWidth)
	if got != name {
		t.Errorf("truncateAgentName(%q): got %q, want unchanged", name, got)
	}
}

func TestTruncateAgentName_Long(t *testing.T) {
	name := "Implement comprehensive test coverage for parser"
	got := truncateAgentName(name, maxAgentNameWidth)

	if ansi.StringWidth(got) > maxAgentNameWidth {
		t.Errorf("truncateAgentName: width %d exceeds max %d, got %q", ansi.StringWidth(got), maxAgentNameWidth, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateAgentName: expected '…' suffix, got %q", got)
	}
}

func TestTruncateAgentName_ExactFit(t *testing.T) {
	// Build a name exactly maxAgentNameWidth chars — should not be truncated.
	name := strings.Repeat("x", maxAgentNameWidth)
	got := truncateAgentName(name, maxAgentNameWidth)
	if got != name {
		t.Errorf("truncateAgentName: exact-fit name should not be truncated, got %q", got)
	}
}
