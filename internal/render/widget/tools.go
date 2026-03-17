package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// highlightSep is the colored separator used for the scrolling ticker position.
// It uses yellowStyle to give the user a visual anchor that advances with each
// new tool call and wraps around the visible separator positions.
var highlightSep = yellowStyle.Render(" | ")

// dimSep is the normal separator used between all non-highlighted tool entries.
var dimSep = dimStyle.Render(" | ")

const maxVisibleTools = 5

// Tools renders running and recently-completed tool invocations as a HUD activity feed.
// Running tools show a yellow category icon + name (default color) + elapsed indicator.
// Completed tools show a dim category icon + name + duration.
// Error tools show the error icon + name + duration + "err" in red.
// Returns an empty WidgetResult when ctx.Transcript is nil or there are no tools to show.
// FgColor is left empty because the widget composes multiple styles internally;
// the renderer passes the pre-styled Text through as-is.
func Tools(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	tools := ctx.Transcript.Tools

	if len(tools) == 0 {
		return WidgetResult{}
	}

	// Reverse the full list so newest tools appear first. This preserves
	// chronological order (Thinking blocks stay at their insertion position
	// rather than being pinned to the front as running tools).
	reversed := make([]model.ToolEntry, len(tools))
	for i, t := range tools {
		reversed[len(tools)-1-i] = t
	}

	visible := reversed
	if len(visible) > maxVisibleTools {
		visible = visible[:maxVisibleTools]
	}

	var parts []string
	for _, t := range visible {
		parts = append(parts, renderToolEntry(icons, t))
	}

	// Compute the highlighted separator position using wrapping ticker logic.
	// DividerOffset is a monotonic counter incremented per tool_use. The
	// highlighted separator cycles through the visible positions so the user
	// has a stable visual anchor that advances with each new tool call.
	numSeps := len(parts) - 1
	if numSeps <= 0 {
		return WidgetResult{Text: joinWithHighlight(parts, -1)}
	}
	highlightIdx := ctx.Transcript.DividerOffset % numSeps

	return WidgetResult{Text: joinWithHighlight(parts, highlightIdx)}
}

// joinWithHighlight joins tool entry parts with separators, highlighting one.
//
// highlightIdx is the 0-based separator position to highlight (0 = between
// parts[0] and parts[1], 1 = between parts[1] and parts[2], etc.).
// A negative value means no separator is highlighted.
// When only one entry is present no separator is emitted at all.
func joinWithHighlight(parts []string, highlightIdx int) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		sep := dimSep
		if i-1 == highlightIdx {
			sep = highlightSep
		}
		out += sep + parts[i]
	}
	return out
}

// renderToolEntry formats a single tool entry according to its state.
func renderToolEntry(icons Icons, t model.ToolEntry) string {
	catIcon := CategoryIcon(icons, t.Category)

	if !t.Completed {
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
