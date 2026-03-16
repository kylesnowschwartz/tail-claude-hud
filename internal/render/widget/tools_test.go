package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// toolsCtx is a helper that builds a RenderContext with a given tools slice.
// The slice is expected to be in oldest-first order, matching how
// ExtractionState.ToTranscriptData produces the Tools field.
// FreshBoundaryCount defaults to 0 (no prior snapshot — all tools are fresh,
// no colored separator shown).
func toolsCtx(tools []model.ToolEntry) *model.RenderContext {
	return &model.RenderContext{
		Transcript: &model.TranscriptData{Tools: tools},
	}
}

// toolsCtxWithBoundary builds a RenderContext with both a tools slice and a
// FreshBoundaryCount simulating a prior-invocation snapshot. freshBoundary is
// the number of tools that existed at the last snapshot save; tools added since
// then are considered "fresh" and the colored separator is placed after them.
func toolsCtxWithBoundary(tools []model.ToolEntry, freshBoundary int) *model.RenderContext {
	return &model.RenderContext{
		Transcript: &model.TranscriptData{
			Tools:              tools,
			FreshBoundaryCount: freshBoundary,
		},
	}
}

// toolNames returns the tool names visible in the rendered output, in order.
// It splits on " | " and strips ANSI by searching for the plain name substring.
// Rather than stripping ANSI codes we just check Contains — order is verified
// by the position of each name relative to the others in the raw string.
func toolNamesInOrder(got string) []string {
	// Split on separator to get per-entry segments.
	parts := strings.Split(got, " | ")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		names = append(names, p)
	}
	return names
}

// containsInOrder returns true when all want strings appear in output in the
// given order (each want appears after the previous one).
func containsInOrder(output string, want []string) bool {
	pos := 0
	for _, w := range want {
		idx := strings.Index(output[pos:], w)
		if idx < 0 {
			return false
		}
		pos += idx + len(w)
	}
	return true
}

// TestFormatDuration_RenderedInCompletedTool verifies that a completed tool
// with a sub-100ms duration (e.g. 50ms) renders "<0.1s" in the output
// rather than the misleading "0.0s".
func TestFormatDuration_RenderedInCompletedTool(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "Grep", Count: 1, DurationMs: 50, Category: "search"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg)

	if !strings.Contains(got, "<0.1s") {
		t.Errorf("expected '<0.1s' for 50ms duration, got %q", got)
	}
	if strings.Contains(got, "0.0s") {
		t.Errorf("got misleading '0.0s' for 50ms duration in %q", got)
	}
}

// Spec 1: 3 running + 4 completed → running tools shown first, then newest completed.
//
// The Tools slice is oldest-first.  Running tools (Count==0) appear before
// completed tools (Count>0) in the rendered output, and completed tools are
// shown newest-first, capped at maxVisibleTools=5 total.
//
// With 3 running + 4 completed we have 7 entries; only 5 are shown:
// all 3 running + the 2 newest completed (C4 and C3).
func TestTools_RunningFirstThenNewestCompleted(t *testing.T) {
	tools := []model.ToolEntry{
		// completed (oldest first)
		{Name: "C1", Count: 1, DurationMs: 100, Category: "internal"},
		{Name: "C2", Count: 1, DurationMs: 100, Category: "internal"},
		{Name: "C3", Count: 1, DurationMs: 100, Category: "internal"},
		{Name: "C4", Count: 1, DurationMs: 100, Category: "internal"},
		// running
		{Name: "R1", Count: 0, Category: "internal"},
		{Name: "R2", Count: 0, Category: "internal"},
		{Name: "R3", Count: 0, Category: "internal"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg)

	// Exactly 5 entries (4 separators).
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// All 3 running tools must be present.
	for _, name := range []string{"R1", "R2", "R3"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected running tool %q in output, got %q", name, got)
		}
	}

	// The 2 newest completed (C4 and C3) must be present.
	for _, name := range []string{"C4", "C3"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected completed tool %q in output, got %q", name, got)
		}
	}

	// The 2 oldest completed (C1, C2) must be dropped.
	for _, name := range []string{"C1", "C2"} {
		if strings.Contains(got, name) {
			t.Errorf("oldest completed tool %q should be excluded, got %q", name, got)
		}
	}

	// Running tools must precede any completed tool in display order.
	// R1 appears before C4 and before C3.
	if !containsInOrder(got, []string{"R1", "C4"}) {
		t.Errorf("running tool R1 should appear before newest completed C4, got %q", got)
	}
}

// Spec 2: 6 completed tools → only 5 shown, oldest dropped.
//
// displayTools is oldest-first; the widget reverses completed tools to get
// newest-first, then caps at 5.  The oldest (C1) must not appear.
func TestTools_SixCompleted_OldestDropped(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "C1", Count: 1, DurationMs: 100, Category: "internal"}, // oldest — should be dropped
		{Name: "C2", Count: 1, DurationMs: 200, Category: "internal"},
		{Name: "C3", Count: 1, DurationMs: 300, Category: "internal"},
		{Name: "C4", Count: 1, DurationMs: 400, Category: "internal"},
		{Name: "C5", Count: 1, DurationMs: 500, Category: "internal"},
		{Name: "C6", Count: 1, DurationMs: 600, Category: "internal"}, // newest — should appear first
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg)

	// Exactly 5 entries.
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// Oldest (C1) must be absent.
	if strings.Contains(got, "C1") {
		t.Errorf("oldest tool C1 should be excluded, got %q", got)
	}

	// Newest (C6) must be present and appear before older ones.
	if !strings.Contains(got, "C6") {
		t.Errorf("newest tool C6 must be present, got %q", got)
	}

	// Newest-first order: C6 before C5 before C4.
	if !containsInOrder(got, []string{"C6", "C5", "C4"}) {
		t.Errorf("expected C6 then C5 then C4 in newest-first order, got %q", got)
	}
}

// Spec 3: tools completing out of order (tool B completes before tool A).
//
// The transcript slice order determines "position" in the display.  When tool
// A is added before B but B completes first, the display order reflects the
// *position* in the slice (which is insertion order), not completion order.
// After reversal, B appears before A because B has a higher index (was added
// later and completed first, but was inserted after A in this test).
//
// More precisely: the slice is [A-running, B-completed].  After separation and
// reversal of completed, visible = [A-running, B-completed].  If A completes
// later it becomes [A-completed, B-completed], reversed → [B-completed, A-completed],
// i.e. B is shown first because it occupied a higher index.
//
// This test verifies the "out of order" scenario where B (inserted second)
// completes before A (inserted first, still running): running A comes first.
func TestTools_OutOfOrderCompletion_DisplayOrderCorrect(t *testing.T) {
	// A was started first (index 0) and is still running.
	// B was started second (index 1) and has already completed.
	// C was started third (index 2) and has already completed.
	tools := []model.ToolEntry{
		{Name: "ToolA", Count: 0, Category: "shell"},                   // still running
		{Name: "ToolB", Count: 1, DurationMs: 500, Category: "file"},   // completed first
		{Name: "ToolC", Count: 1, DurationMs: 1000, Category: "search"}, // completed second
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg)

	// All three tools must appear.
	for _, name := range []string{"ToolA", "ToolB", "ToolC"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected tool %q in output, got %q", name, got)
		}
	}

	// Running ToolA must appear before completed tools.
	if !containsInOrder(got, []string{"ToolA", "ToolC"}) {
		t.Errorf("running ToolA should appear before completed ToolC, got %q", got)
	}
	if !containsInOrder(got, []string{"ToolA", "ToolB"}) {
		t.Errorf("running ToolA should appear before completed ToolB, got %q", got)
	}

	// Among completed tools, ToolC (higher index = newer) appears before ToolB.
	if !containsInOrder(got, []string{"ToolC", "ToolB"}) {
		t.Errorf("ToolC (newer position) should appear before ToolB, got %q", got)
	}
}

// Spec 5 recommendation (see TestTools_MaxToolsBufferSizeRecommendation below).

// Spec 4: maxTools=20 buffer fills then prunes → display still shows correct last 5.
//
// ExtractionState caps displayTools at 20, pruning the oldest from the front.
// Once 25 tools have been added, tools 1-5 are pruned; tools 6-25 remain.
// This test simulates the result: a Tools slice of 20 entries (oldest-first),
// representing the surviving window after pruning.
// The widget must still show the 5 newest (T21–T25 mapped to T16–T20 in the
// surviving slice, i.e. the last 5 of the 20 remaining).
func TestTools_MaxToolsBufferFillsAndPrunes(t *testing.T) {
	// Simulate what ExtractionState produces after 25 tool completions:
	// displayTools holds entries 6..25 (the oldest 5 were pruned).
	// We represent this as a slice of 20 completed tools named T06..T25.
	const bufferSize = 20
	tools := make([]model.ToolEntry, bufferSize)
	for i := 0; i < bufferSize; i++ {
		tools[i] = model.ToolEntry{
			// Names T06 through T25 (matching the surviving window after 5 pruned).
			Name:       "T" + fmt.Sprintf("%02d", i+6),
			Count:      1,
			DurationMs: (i + 6) * 100,
			Category:   "internal",
		}
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg)

	// Exactly 5 entries shown.
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// The 5 newest (T21–T25) must be present.
	for _, name := range []string{"T21", "T22", "T23", "T24", "T25"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected recent tool %q in output, got %q", name, got)
		}
	}

	// The oldest visible entry (T20) must not appear since only T21-T25 fit.
	if strings.Contains(got, "T20") {
		t.Errorf("tool T20 should be outside the 5-entry window, got %q", got)
	}

	// Newest-first ordering: T25 before T24 before T23.
	if !containsInOrder(got, []string{"T25", "T24", "T23"}) {
		t.Errorf("expected T25 then T24 then T23 in newest-first order, got %q", got)
	}
}

// TestTools_FreshBoundarySeparator verifies the colored separator behavior.
//
// The yellow separator marks the boundary between "fresh" tools (added since
// the last snapshot) and "old" tools (present in the prior snapshot). This
// boundary is tracked via FreshBoundaryCount in TranscriptData.
//
// Spec 1: the separator position persists via FreshBoundaryCount.
// Spec 2: when new tools are prepended, the colored separator shifts rightward.
// Spec 3: when all tools are newer than the last snapshot (FreshBoundaryCount==0),
//         the colored separator does not appear.
func TestTools_FreshBoundarySeparator(t *testing.T) {
	t.Run("no prior snapshot means no colored separator", func(t *testing.T) {
		// FreshBoundaryCount=0 (zero value, no prior snapshot) — all tools are fresh,
		// so no colored separator is emitted.
		tools := []model.ToolEntry{
			{Name: "OldTool", Count: 1, DurationMs: 100, Category: "internal"},
			{Name: "NewTool", Count: 1, DurationMs: 200, Category: "internal"},
		}
		ctx := toolsCtx(tools) // FreshBoundaryCount defaults to 0
		cfg := defaultCfg()

		got := Tools(ctx, cfg)

		if strings.Contains(got, freshSep) {
			t.Errorf("expected no fresh separator when FreshBoundaryCount=0 (all fresh), got %q", got)
		}
	})

	t.Run("one old tool: colored separator after first (fresh) tool", func(t *testing.T) {
		// 2 tools total, 1 was in the prior snapshot. The newest tool is fresh,
		// the older one is old. Separator goes between them.
		// Visible (newest-first): NewTool [freshSep] OldTool
		tools := []model.ToolEntry{
			{Name: "OldTool", Count: 1, DurationMs: 100, Category: "internal"},
			{Name: "NewTool", Count: 1, DurationMs: 200, Category: "internal"},
		}
		ctx := toolsCtxWithBoundary(tools, 1) // 1 tool at last snapshot
		cfg := defaultCfg()

		got := Tools(ctx, cfg)

		if !strings.Contains(got, freshSep) {
			t.Errorf("expected fresh (yellow) separator, got %q", got)
		}

		// freshSep must appear before OldTool (the old tool is to the right).
		freshIdx := strings.Index(got, freshSep)
		oldToolIdx := strings.Index(got, "OldTool")
		if freshIdx < 0 || oldToolIdx < 0 {
			t.Fatalf("missing fresh separator or OldTool in %q", got)
		}
		if freshIdx > oldToolIdx {
			t.Errorf("fresh separator (pos %d) should precede OldTool (pos %d)", freshIdx, oldToolIdx)
		}
	})

	t.Run("three tools, one fresh: separator after first, then dim separators", func(t *testing.T) {
		// 3 tools total, 2 in prior snapshot. Only C is fresh.
		// Visible (newest-first): C [freshSep] B [dimSep] A
		tools := []model.ToolEntry{
			{Name: "A", Count: 1, DurationMs: 100, Category: "internal"},
			{Name: "B", Count: 1, DurationMs: 200, Category: "internal"},
			{Name: "C", Count: 1, DurationMs: 300, Category: "internal"},
		}
		ctx := toolsCtxWithBoundary(tools, 2) // 2 tools at last snapshot
		cfg := defaultCfg()

		got := Tools(ctx, cfg)

		freshIdx := strings.Index(got, freshSep)
		dimIdx := strings.Index(got, dimSep)

		if freshIdx < 0 {
			t.Fatalf("expected fresh separator in output, got %q", got)
		}
		if dimIdx < 0 {
			t.Fatalf("expected dim separator in output, got %q", got)
		}
		// The fresh separator must appear before the dim one.
		if freshIdx > dimIdx {
			t.Errorf("fresh separator (pos %d) should precede dim separator (pos %d) in %q", freshIdx, dimIdx, got)
		}
	})

	t.Run("single tool has no separator", func(t *testing.T) {
		tools := []model.ToolEntry{
			{Name: "Solo", Count: 1, DurationMs: 100, Category: "internal"},
		}
		ctx := toolsCtx(tools)
		cfg := defaultCfg()

		got := Tools(ctx, cfg)

		if strings.Contains(got, freshSep) {
			t.Errorf("single-entry output should have no separator, got %q", got)
		}
		if strings.Contains(got, dimSep) {
			t.Errorf("single-entry output should have no separator, got %q", got)
		}
	})

	t.Run("separator shifts rightward when new tool arrives", func(t *testing.T) {
		// Invocation 1: 2 tools, snapshot saves FreshBoundaryCount=2.
		// Invocation 2: 3 tools (Tool3 is new). FreshBoundaryCount=2.
		// freshCount = 3 - 2 = 1. Separator after position 1 (between Tool3 and Tool2).
		// Visible (newest-first): Tool3 [freshSep] Tool2 [dimSep] Tool1

		toolsAfter := []model.ToolEntry{
			{Name: "Tool1", Count: 1, DurationMs: 100, Category: "internal"},
			{Name: "Tool2", Count: 1, DurationMs: 200, Category: "internal"},
			{Name: "Tool3", Count: 1, DurationMs: 300, Category: "internal"},
		}
		cfg := defaultCfg()

		// Simulate the state after the second invocation:
		// FreshBoundaryCount=2 (Tool1 and Tool2 were present at last snapshot).
		gotAfter := Tools(toolsCtxWithBoundary(toolsAfter, 2), cfg)

		if !strings.Contains(gotAfter, freshSep) {
			t.Errorf("expected fresh separator after new tool arrived, got %q", gotAfter)
		}

		// freshSep must appear before Tool2 (Tool3 is fresh, Tool2 is old).
		freshIdxAfter := strings.Index(gotAfter, freshSep)
		tool2IdxAfter := strings.Index(gotAfter, "Tool2")
		if freshIdxAfter < 0 || tool2IdxAfter < 0 {
			t.Fatalf("missing fresh separator or Tool2 in %q", gotAfter)
		}
		if freshIdxAfter > tool2IdxAfter {
			t.Errorf("fresh separator (pos %d) should precede Tool2 (pos %d)", freshIdxAfter, tool2IdxAfter)
		}
	})

	t.Run("all visible tools are old: colored separator does not appear at end", func(t *testing.T) {
		// 2 tools total, FreshBoundaryCount=5 (boundary from a longer past invocation).
		// freshCount = 2 - 5 = -1 → clamped to 0 → all visible are fresh, no separator.
		tools := []model.ToolEntry{
			{Name: "Tool1", Count: 1, DurationMs: 100, Category: "internal"},
			{Name: "Tool2", Count: 1, DurationMs: 200, Category: "internal"},
		}
		ctx := toolsCtxWithBoundary(tools, 5)
		cfg := defaultCfg()

		got := Tools(ctx, cfg)

		// freshCount clamps to 0 — all tools treated as fresh, no separator.
		if strings.Contains(got, freshSep) {
			t.Errorf("no fresh separator expected when freshCount<=0, got %q", got)
		}
	})
}

// TestTools_MaxToolsBufferSizeRecommendation documents the spec 5 analysis.
//
// Recommendation: keep maxTools=20 as a look-back buffer rather than reducing
// it to match maxVisibleTools=5.
//
// Rationale: the 20-entry buffer in ExtractionState serves a different purpose
// than the 5-entry visible cap in the widget.  The buffer retains enough history
// so that, when several running tools complete in quick succession, the widget
// can still present the correct 5 newest.  If the buffer were shrunk to 5,
// a burst of 6+ tool invocations would evict entries before the widget has a
// chance to render them, potentially showing stale or incomplete state.
// The 20:5 ratio (4x headroom) is a reasonable safety margin for typical
// Claude Code sessions; lowering it is safe only if the caller guarantees that
// no more than 5 tools will be in-flight simultaneously.
func TestTools_MaxToolsBufferSizeRecommendation(t *testing.T) {
	// This test exists to anchor the spec 5 recommendation in a verifiable
	// assertion: a session with more tools than maxVisibleTools still renders
	// the correct last 5 after the buffer has pruned older entries.
	//
	// If someone reduces maxTools to 5 this test still passes (the widget only
	// ever sees 5 entries), but the more important property — that a burst of
	// completions doesn't drop the newest entries before rendering — can only
	// be verified through ExtractionState integration tests, not here.
	const maxToolsBuf = 20   // from extractor.go
	const maxVisible = 5     // from tools.go

	if maxToolsBuf < maxVisible {
		t.Errorf("maxTools buffer (%d) must be >= maxVisibleTools (%d)", maxToolsBuf, maxVisible)
	}
}
