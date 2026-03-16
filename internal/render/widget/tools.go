package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// freshSep is the colored separator placed after the fresh tools to mark the
// boundary between tools added since the last snapshot and older ones.
// It uses yellowStyle (matching running-tool color) to signal the "fresh boundary":
// everything to its left is newer than everything to its right.
var freshSep = yellowStyle.Render(" | ")

// dimSep is the normal separator used between all non-fresh tool entries.
var dimSep = dimStyle.Render(" | ")

const maxVisibleTools = 5

// Tools renders running and recently-completed tool invocations as a HUD activity feed.
// Running tools show a yellow category icon + name (default color) + elapsed indicator.
// Completed tools show a dim category icon + name + duration.
// Error tools show the error icon + name + duration + "err" in red.
// Returns "" when ctx.Transcript is nil or there are no tools to show.
func Tools(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Transcript == nil {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)
	tools := ctx.Transcript.Tools

	if len(tools) == 0 {
		return ""
	}

	// Separate running (Count==0) from completed/error (Count>0).
	var running []model.ToolEntry
	var completed []model.ToolEntry
	for _, t := range tools {
		if t.Count == 0 {
			running = append(running, t)
		} else {
			completed = append(completed, t)
		}
	}

	// Build the visible list: running tools take priority, then completed newest-first.
	reversed := make([]model.ToolEntry, len(completed))
	for i, t := range completed {
		reversed[len(completed)-1-i] = t
	}

	var visible []model.ToolEntry
	visible = append(visible, running...)
	visible = append(visible, reversed...)
	if len(visible) > maxVisibleTools {
		visible = visible[:maxVisibleTools]
	}

	var parts []string
	for _, t := range visible {
		parts = append(parts, renderToolEntry(icons, t))
	}

	// Compute the fresh boundary index for the visible slice.
	//
	// displayTools is oldest-first; visible is built newest-first (running first,
	// then reversed completed). FreshBoundaryCount is the number of tools that
	// existed at the last snapshot save. Tools beyond that count are "fresh".
	//
	// freshCount = total tools now - tools at last snapshot
	// These fresh tools occupy positions 0..freshCount-1 in the visible slice
	// (because the visible slice is newest-first). The colored separator goes
	// after the last fresh entry, i.e. at index freshCount.
	//
	// When freshCount >= len(visible), all visible tools are fresh — no separator.
	// When FreshBoundaryCount == 0 (no prior snapshot), treat all as fresh — no separator.
	totalTools := len(ctx.Transcript.Tools)
	freshCount := totalTools - ctx.Transcript.FreshBoundaryCount
	if freshCount < 0 {
		freshCount = 0
	}
	// Cap at visible length — can't place separator beyond the list.
	if freshCount > len(visible) {
		freshCount = len(visible)
	}

	return joinWithFreshBoundary(parts, freshCount)
}

// joinWithFreshBoundary joins tool entry parts with colored separators.
//
// freshBoundaryIdx is the position where the colored (yellow) separator is
// inserted: between parts[freshBoundaryIdx-1] and parts[freshBoundaryIdx].
// Parts before that index are "fresh" (added since the last snapshot); parts
// at and after it are "old". All other separators use the dim style.
//
// If freshBoundaryIdx <= 0 or >= len(parts), no colored separator is emitted
// (either all tools are fresh, or there are no old tools to mark the boundary
// against). When only one entry is present no separator is emitted at all.
func joinWithFreshBoundary(parts []string, freshBoundaryIdx int) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		sep := dimSep
		if i == freshBoundaryIdx {
			sep = freshSep
		}
		out += sep + parts[i]
	}
	return out
}

// renderToolEntry formats a single tool entry according to its state.
func renderToolEntry(icons Icons, t model.ToolEntry) string {
	catIcon := CategoryIcon(icons, t.Category)

	if t.Count == 0 {
		// Running: yellow category icon only; name uses default foreground to match
		// the completed-tool pattern where only the icon carries color.
		icon := yellowStyle.Render(catIcon)
		return fmt.Sprintf("%s %s", icon, t.Name)
	}

	if t.HasError {
		// Error: red category icon + name + duration.
		icon := redStyle.Render(catIcon)
		name := redStyle.Render(t.Name)
		dur := redStyle.Render(formatDuration(t.DurationMs))
		return fmt.Sprintf("%s %s %s", icon, name, dur)
	}

	// Completed: green category icon + name + duration.
	icon := greenStyle.Render(catIcon)
	name := dimStyle.Render(t.Name)
	dur := dimStyle.Render(formatDuration(t.DurationMs))
	return fmt.Sprintf("%s %s %s", icon, name, dur)
}

// formatDuration converts a millisecond duration into a compact human-readable string.
//
//   - <= 0ms:             "0.0s"  (genuinely instant or unknown)
//   - 1ms – 99ms:        "<0.1s" (sub-100ms; avoids misleading "0.0s" for real durations)
//   - 100ms – 999ms:     "0.Xs"  (tenths of a second)
//   - 1000ms – 59999ms:  "Xs" or "X.Ys" (seconds, optional tenth)
//   - >= 60000ms:         "Xm Ys"
func formatDuration(ms int) string {
	if ms <= 0 {
		return "0.0s"
	}
	if ms < 100 {
		return "<0.1s"
	}
	if ms < 1000 {
		return fmt.Sprintf("0.%ds", ms/100)
	}
	if ms < 60000 {
		secs := ms / 1000
		frac := (ms % 1000) / 100
		if frac == 0 {
			return fmt.Sprintf("%ds", secs)
		}
		return fmt.Sprintf("%d.%ds", secs, frac)
	}
	mins := ms / 60000
	secs := (ms % 60000) / 1000
	return fmt.Sprintf("%dm %ds", mins, secs)
}

// formatTokenCost formats a token count as a compact cost string:
//   - < 1000:    printed as-is (e.g. "500")
//   - 1000–99999: "X.Yk" (one decimal place)
//   - >= 100000:  "Xk" (no decimal)
func formatTokenCost(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 100000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}
