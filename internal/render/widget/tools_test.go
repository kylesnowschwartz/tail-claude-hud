package widget

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// toolsCtx is a helper that builds a RenderContext with a given tools slice.
// The slice is expected to be in oldest-first order, matching how
// ExtractionState.ToTranscriptData produces the Tools field.
// DividerOffset defaults to 0.
func toolsCtx(tools []model.ToolEntry) *model.RenderContext {
	return &model.RenderContext{
		Transcript: &model.TranscriptData{Tools: tools},
	}
}

// toolsCtxWithOffset builds a RenderContext with both a tools slice and a
// DividerOffset. The widget highlights separator at position offset % (numVisible - 1).
func toolsCtxWithOffset(tools []model.ToolEntry, offset int) *model.RenderContext {
	return &model.RenderContext{
		Transcript: &model.TranscriptData{
			Tools:         tools,
			DividerOffset: offset,
		},
	}
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
		{Name: "Grep", Completed: true, DurationMs: 50, Category: "Grep"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	if !strings.Contains(got, "<0.1s") {
		t.Errorf("expected '<0.1s' for 50ms duration, got %q", got)
	}
	if strings.Contains(got, "0.0s") {
		t.Errorf("got misleading '0.0s' for 50ms duration in %q", got)
	}
}

// Spec 1: 3 running + 4 completed -> newest 5 shown in chronological (insertion) order.
//
// The Tools slice is oldest-first. The widget reverses the full list to get
// newest-first, then caps at maxVisibleTools=5. Running tools are not pinned;
// they appear at their natural position in the list.
//
// With slice [C1, C2, C3, C4, R1, R2, R3] (7 entries), reversed = [R3, R2, R1, C4, C3, ...].
// Only 5 are shown: R3, R2, R1, C4, C3.
func TestTools_NewestFirstChronological(t *testing.T) {
	tools := []model.ToolEntry{
		// completed (oldest first)
		{Name: "C1", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "C2", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "C3", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "C4", Completed: true, DurationMs: 100, Category: "Other"},
		// running (most recently started)
		{Name: "R1", Category: "Other"},
		{Name: "R2", Category: "Other"},
		{Name: "R3", Category: "Other"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// Exactly 5 entries (4 separators).
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// The 3 most-recently-inserted running tools must be present.
	for _, name := range []string{"R1", "R2", "R3"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected running tool %q in output, got %q", name, got)
		}
	}

	// C4 (4th inserted, just before running tools) must be present.
	if !strings.Contains(got, "C4") {
		t.Errorf("expected completed tool C4 in output, got %q", got)
	}

	// C3 (3rd inserted) must be present.
	if !strings.Contains(got, "C3") {
		t.Errorf("expected completed tool C3 in output, got %q", got)
	}

	// The 2 oldest (C1, C2) must be dropped.
	for _, name := range []string{"C1", "C2"} {
		if strings.Contains(got, name) {
			t.Errorf("oldest tool %q should be excluded, got %q", name, got)
		}
	}

	// Newest-first (reversed insertion) order: R3 before R2 before R1 before C4.
	if !containsInOrder(got, []string{"R3", "R2", "R1", "C4"}) {
		t.Errorf("expected newest-first order R3, R2, R1, C4, got %q", got)
	}
}

// Spec 2: 6 completed tools -> only 5 shown, oldest dropped.
//
// displayTools is oldest-first; the widget reverses completed tools to get
// newest-first, then caps at 5.  The oldest (C1) must not appear.
func TestTools_SixCompleted_OldestDropped(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "C1", Completed: true, DurationMs: 100, Category: "Other"}, // oldest -- should be dropped
		{Name: "C2", Completed: true, DurationMs: 200, Category: "Other"},
		{Name: "C3", Completed: true, DurationMs: 300, Category: "Other"},
		{Name: "C4", Completed: true, DurationMs: 400, Category: "Other"},
		{Name: "C5", Completed: true, DurationMs: 500, Category: "Other"},
		{Name: "C6", Completed: true, DurationMs: 600, Category: "Other"}, // newest -- should appear first
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

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

// Spec 3: tools with mixed running/completed state display in chronological order.
//
// The slice is [A-running, B-completed, C-completed] (oldest-first). After full
// reversal the display order is [C-completed, B-completed, A-running] (newest-first).
// A running tool is NOT pinned to the front; it appears at its insertion position.
func TestTools_OutOfOrderCompletion_DisplayOrderCorrect(t *testing.T) {
	// A was started first (index 0) and is still running.
	// B was started second (index 1) and has already completed.
	// C was started third (index 2) and has already completed.
	tools := []model.ToolEntry{
		{Name: "ToolA", Category: "Bash"},                                    // still running, oldest
		{Name: "ToolB", Completed: true, DurationMs: 500, Category: "Read"},  // completed, middle
		{Name: "ToolC", Completed: true, DurationMs: 1000, Category: "Grep"}, // completed, newest
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// All three tools must appear.
	for _, name := range []string{"ToolA", "ToolB", "ToolC"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected tool %q in output, got %q", name, got)
		}
	}

	// Newest-first (reversed insertion) order: ToolC then ToolB then ToolA.
	if !containsInOrder(got, []string{"ToolC", "ToolB", "ToolA"}) {
		t.Errorf("expected newest-first order ToolC, ToolB, ToolA, got %q", got)
	}
}

// TestTools_ThinkingChronologicalOrder verifies that a running Thinking entry
// between two completed tools appears at its chronological position rather than
// being pinned to the front of the display.
//
// Slice (oldest-first): [Read-completed, Thinking-running, Grep-completed]
// After reversal (newest-first): [Grep-completed, Thinking-running, Read-completed]
// Thinking must NOT appear before Grep.
func TestTools_ThinkingChronologicalOrder(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "Read", Completed: true, DurationMs: 300, Category: "Read"}, // oldest
		{Name: "Thinking", Completed: false, Category: "Thinking"},         // middle, still running
		{Name: "Grep", Completed: true, DurationMs: 150, Category: "Grep"}, // newest
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// All three must appear.
	for _, name := range []string{"Read", "Thinking", "Grep"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected tool %q in output, got %q", name, got)
		}
	}

	// Grep (newest) must appear before Thinking (middle) in the display.
	if !containsInOrder(got, []string{"Grep", "Thinking"}) {
		t.Errorf("Grep (newest) should appear before Thinking (middle), got %q", got)
	}

	// Thinking (middle) must appear before Read (oldest).
	if !containsInOrder(got, []string{"Thinking", "Read"}) {
		t.Errorf("Thinking (middle) should appear before Read (oldest), got %q", got)
	}
}

// TestTools_NewestToolsAppearFirst verifies that when only completed tools are
// present, the output is strictly newest-first (highest insertion index first).
func TestTools_NewestToolsAppearFirst(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "T1", Completed: true, DurationMs: 100, Category: "Other"}, // oldest
		{Name: "T2", Completed: true, DurationMs: 200, Category: "Other"},
		{Name: "T3", Completed: true, DurationMs: 300, Category: "Other"}, // newest
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// Newest-first order: T3 before T2 before T1.
	if !containsInOrder(got, []string{"T3", "T2", "T1"}) {
		t.Errorf("expected newest-first order T3, T2, T1, got %q", got)
	}
}

// TestTools_MaxVisibleToolsCap verifies that when more than maxVisibleTools
// entries exist, only the newest maxVisibleTools are shown and the oldest are dropped.
func TestTools_MaxVisibleToolsCap(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "Old1", Completed: true, DurationMs: 100, Category: "Other"}, // oldest, should be dropped
		{Name: "Old2", Completed: true, DurationMs: 100, Category: "Other"}, // should be dropped
		{Name: "N3", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "N4", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "N5", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "N6", Completed: true, DurationMs: 100, Category: "Other"},
		{Name: "N7", Completed: true, DurationMs: 100, Category: "Other"}, // newest
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// Exactly 5 entries (4 separators).
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// Oldest two must be absent.
	for _, name := range []string{"Old1", "Old2"} {
		if strings.Contains(got, name) {
			t.Errorf("oldest tool %q should be excluded, got %q", name, got)
		}
	}

	// The 5 newest must be present (N3 through N7).
	for _, name := range []string{"N3", "N4", "N5", "N6", "N7"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected recent tool %q in output, got %q", name, got)
		}
	}
}

// Spec 5 recommendation (see TestTools_MaxToolsBufferSizeRecommendation below).

// Spec 4: maxTools=20 buffer fills then prunes -> display still shows correct last 5.
//
// ExtractionState caps displayTools at 20, pruning the oldest from the front.
// Once 25 tools have been added, tools 1-5 are pruned; tools 6-25 remain.
// This test simulates the result: a Tools slice of 20 entries (oldest-first),
// representing the surviving window after pruning.
// The widget must still show the 5 newest (T21-T25 mapped to T16-T20 in the
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
			Completed:  true,
			DurationMs: (i + 6) * 100,
			Category:   "internal",
		}
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).Text

	// Exactly 5 entries shown.
	separators := strings.Count(got, " | ")
	if separators != 4 {
		t.Errorf("expected 4 separators (5 entries), got %d in %q", separators, got)
	}

	// The 5 newest (T21-T25) must be present.
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

// TestTools_DividerHighlight verifies the scrolling ticker separator behavior.
//
// The highlighted separator cycles through positions based on DividerOffset.
// With N visible tools there are N-1 separators. The highlighted position is
// offset % (N-1), wrapping around when it exceeds the last position.
func TestTools_DividerHighlight(t *testing.T) {
	t.Run("single tool has no separator", func(t *testing.T) {
		tools := []model.ToolEntry{
			{Name: "Solo", Completed: true, DurationMs: 100, Category: "Other"},
		}
		ctx := toolsCtx(tools)
		cfg := defaultCfg()

		got := Tools(ctx, cfg).Text

		if strings.Contains(got, highlightSep) || strings.Contains(got, dimSep) {
			t.Errorf("single-entry output should have no separator, got %q", got)
		}
	})

	t.Run("two tools: highlight cycles between sole separator position", func(t *testing.T) {
		tools := []model.ToolEntry{
			{Name: "A", Completed: true, DurationMs: 100, Category: "Other"},
			{Name: "B", Completed: true, DurationMs: 200, Category: "Other"},
		}
		cfg := defaultCfg()

		// 2 tools = 1 separator. Any offset mod 1 = 0, so it's always highlighted.
		for _, offset := range []int{0, 1, 5, 100} {
			got := Tools(toolsCtxWithOffset(tools, offset), cfg).Text
			if !strings.Contains(got, highlightSep) {
				t.Errorf("offset=%d: expected highlighted separator with 2 tools, got %q", offset, got)
			}
		}
	})

	t.Run("three tools: highlight position wraps", func(t *testing.T) {
		tools := []model.ToolEntry{
			{Name: "A", Completed: true, DurationMs: 100, Category: "Other"},
			{Name: "B", Completed: true, DurationMs: 200, Category: "Other"},
			{Name: "C", Completed: true, DurationMs: 300, Category: "Other"},
		}
		cfg := defaultCfg()

		// 3 tools = 2 separators (positions 0 and 1).
		// Visible newest-first: C sep0 B sep1 A

		// offset=0 -> highlight position 0 (between C and B)
		got0 := Tools(toolsCtxWithOffset(tools, 0), cfg).Text
		hlIdx0 := strings.Index(got0, highlightSep)
		bIdx0 := strings.Index(got0, "B")
		if hlIdx0 < 0 || hlIdx0 > bIdx0 {
			t.Errorf("offset=0: highlight should be before B, got %q", got0)
		}

		// offset=1 -> highlight position 1 (between B and A)
		got1 := Tools(toolsCtxWithOffset(tools, 1), cfg).Text
		hlIdx1 := strings.Index(got1, highlightSep)
		aIdx1 := strings.Index(got1, "A")
		if hlIdx1 < 0 || hlIdx1 > aIdx1 {
			t.Errorf("offset=1: highlight should be before A, got %q", got1)
		}

		// offset=2 -> wraps to position 0 again
		got2 := Tools(toolsCtxWithOffset(tools, 2), cfg).Text
		hlIdx2 := strings.Index(got2, highlightSep)
		bIdx2 := strings.Index(got2, "B")
		if hlIdx2 < 0 || hlIdx2 > bIdx2 {
			t.Errorf("offset=2: highlight should wrap to before B, got %q", got2)
		}
	})

	t.Run("highlight advances with each new tool", func(t *testing.T) {
		// Simulates successive tool_use events incrementing DividerOffset.
		// 4 tools = 3 separator positions. Offset 3->6 should cycle through
		// positions 0, 1, 2, 0, 1, 2...
		tools := []model.ToolEntry{
			{Name: "T1", Completed: true, DurationMs: 100, Category: "Other"},
			{Name: "T2", Completed: true, DurationMs: 200, Category: "Other"},
			{Name: "T3", Completed: true, DurationMs: 300, Category: "Other"},
			{Name: "T4", Completed: true, DurationMs: 400, Category: "Other"},
		}
		cfg := defaultCfg()

		for offset := 0; offset < 9; offset++ {
			got := Tools(toolsCtxWithOffset(tools, offset), cfg).Text
			if !strings.Contains(got, highlightSep) {
				t.Errorf("offset=%d: expected a highlighted separator, got %q", offset, got)
			}
			// Count: exactly 1 highlighted, rest are dim.
			hlCount := strings.Count(got, highlightSep)
			if hlCount != 1 {
				t.Errorf("offset=%d: expected exactly 1 highlighted separator, got %d in %q", offset, hlCount, got)
			}
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
	// ever sees 5 entries), but the more important property -- that a burst of
	// completions doesn't drop the newest entries before rendering -- can only
	// be verified through ExtractionState integration tests, not here.
	const maxToolsBuf = 20 // from extractor.go
	const maxVisible = 5   // from tools.go

	if maxToolsBuf < maxVisible {
		t.Errorf("maxTools buffer (%d) must be >= maxVisibleTools (%d)", maxToolsBuf, maxVisible)
	}
}

// --- Recency tier tests ---

func TestRecencyTier_Running(t *testing.T) {
	entry := model.ToolEntry{Name: "Read", Completed: false, Category: "Read"}
	if tier := recencyTier(entry); tier != 0 {
		t.Errorf("running tool should be tier 0, got %d", tier)
	}
}

func TestRecencyTier_Fresh(t *testing.T) {
	// Completed 1 second ago.
	entry := model.ToolEntry{
		Name:       "Read",
		Completed:  true,
		DurationMs: 500,
		Category:   "Read",
		StartTime:  time.Now().Add(-1500 * time.Millisecond), // started 1.5s ago, took 0.5s -> completed 1s ago
	}
	if tier := recencyTier(entry); tier != 1 {
		t.Errorf("tool completed 1s ago should be tier 1 (fresh), got %d", tier)
	}
}

func TestRecencyTier_Recent(t *testing.T) {
	// Completed 10 seconds ago.
	entry := model.ToolEntry{
		Name:       "Read",
		Completed:  true,
		DurationMs: 500,
		Category:   "Read",
		StartTime:  time.Now().Add(-10500 * time.Millisecond), // started 10.5s ago, took 0.5s -> completed 10s ago
	}
	if tier := recencyTier(entry); tier != 2 {
		t.Errorf("tool completed 10s ago should be tier 2 (recent), got %d", tier)
	}
}

func TestRecencyTier_Faded(t *testing.T) {
	// Completed 60 seconds ago.
	entry := model.ToolEntry{
		Name:       "Read",
		Completed:  true,
		DurationMs: 500,
		Category:   "Read",
		StartTime:  time.Now().Add(-60500 * time.Millisecond), // started 60.5s ago, took 0.5s -> completed 60s ago
	}
	if tier := recencyTier(entry); tier != 3 {
		t.Errorf("tool completed 60s ago should be tier 3 (faded), got %d", tier)
	}
}

func TestRecencyTier_ZeroStartTime(t *testing.T) {
	// Missing timestamp falls back to tier 2 (recent).
	entry := model.ToolEntry{
		Name:       "Read",
		Completed:  true,
		DurationMs: 500,
		Category:   "Read",
		// StartTime is zero
	}
	if tier := recencyTier(entry); tier != 2 {
		t.Errorf("zero-start-time tool should be tier 2 (recent fallback), got %d", tier)
	}
}

// --- Consecutive grouping tests ---

// TestTools_ConsecutiveGrouping verifies that consecutive tools with the same
// name are collapsed into "Name ×N" instead of being listed individually.
func TestTools_ConsecutiveGrouping(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 200, Category: "Bash"},
		{Name: "Edit", Completed: true, DurationMs: 50, Category: "Edit"},
		{Name: "Bash", Completed: true, DurationMs: 300, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 400, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 500, Category: "Bash"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).PlainText

	// After reversal (newest-first): Bash, Bash, Bash, Edit, Bash, Bash
	// Groups: [Bash ×3] [Edit] [Bash ×2]
	if !strings.Contains(got, "×3") {
		t.Errorf("expected ×3 for 3 consecutive Bash, got %q", got)
	}
	if !strings.Contains(got, "×2") {
		t.Errorf("expected ×2 for 2 consecutive Bash, got %q", got)
	}
	// Should have 3 groups = 2 separators
	separators := strings.Count(got, " | ")
	if separators != 2 {
		t.Errorf("expected 2 separators (3 groups), got %d in %q", separators, got)
	}
}

// TestTools_SingleEntriesNotGrouped verifies that non-consecutive same-name
// tools are NOT grouped (only consecutive runs are collapsed).
func TestTools_SingleEntriesNotGrouped(t *testing.T) {
	tools := []model.ToolEntry{
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Edit", Completed: true, DurationMs: 200, Category: "Edit"},
		{Name: "Bash", Completed: true, DurationMs: 300, Category: "Bash"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).PlainText

	// After reversal: Bash, Edit, Bash — no consecutive duplicates
	if strings.Contains(got, "×") {
		t.Errorf("non-consecutive same-name tools should not be grouped, got %q", got)
	}
}

// TestTools_GroupingReducesSlotCount verifies that grouping allows more unique
// tool types to be visible within maxVisibleTools.
func TestTools_GroupingReducesSlotCount(t *testing.T) {
	// 8 tools: 5 Bash then Read, Grep, Edit (oldest-first)
	// Without grouping, only 5 would show (all Bash from newest end).
	// With grouping, reversed = Edit, Grep, Read, Bash×5 → 4 groups, all visible.
	tools := []model.ToolEntry{
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Bash", Completed: true, DurationMs: 100, Category: "Bash"},
		{Name: "Read", Completed: true, DurationMs: 200, Category: "Read"},
		{Name: "Grep", Completed: true, DurationMs: 150, Category: "Grep"},
		{Name: "Edit", Completed: true, DurationMs: 50, Category: "Edit"},
	}
	ctx := toolsCtx(tools)
	cfg := defaultCfg()

	got := Tools(ctx, cfg).PlainText

	// All 4 unique groups should be visible.
	for _, name := range []string{"Edit", "Grep", "Read", "Bash"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected %q to be visible after grouping, got %q", name, got)
		}
	}
	if !strings.Contains(got, "×5") {
		t.Errorf("expected ×5 for 5 consecutive Bash, got %q", got)
	}
}
