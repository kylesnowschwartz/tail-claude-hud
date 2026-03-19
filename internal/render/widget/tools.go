package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// highlightSep is the colored separator used for the scrolling ticker position.
// It uses yellowStyle to give the user a visual anchor that advances with each
// new tool call and wraps around the visible separator positions.
var highlightSep = yellowStyle.Render(" | ")

// dimSep is the normal separator used between all non-highlighted tool entries.
var dimSep = DimStyle.Render(" | ")

const maxVisibleTools = 5

// Tools renders running and recently-completed tool invocations as a HUD activity feed.
// Running tools show a yellow category icon + name (default color).
// Completed tools are styled by recency tier: fresh (<5s), recent (5-30s), faded (>30s).
// Error tools show the error icon + name + duration in red regardless of age.
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
	var plainParts []string
	for _, t := range visible {
		parts = append(parts, renderToolEntry(icons, t))
		plainParts = append(plainParts, renderToolEntryPlain(icons, t))
	}

	// Compute the highlighted separator position using wrapping ticker logic.
	// DividerOffset is a monotonic counter incremented per tool_use. The
	// highlighted separator cycles through the visible positions so the user
	// has a stable visual anchor that advances with each new tool call.
	numSeps := len(parts) - 1
	if numSeps <= 0 {
		return WidgetResult{
			Text:      joinWithHighlight(parts, -1),
			PlainText: joinPlain(plainParts),
			FgColor:   "",
		}
	}
	highlightIdx := ctx.Transcript.DividerOffset % numSeps

	return WidgetResult{
		Text:      joinWithHighlight(parts, highlightIdx),
		PlainText: joinPlain(plainParts),
		FgColor:   "",
	}
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

// joinPlain joins plain-text parts with " | " (unstyled).
func joinPlain(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// recencyTier classifies a completed tool by how recently it finished.
// Returns:
//
//	0 — running (not completed)
//	1 — fresh: completed less than 5 seconds ago
//	2 — recent: completed 5-30 seconds ago (or missing timestamp)
//	3 — faded: completed more than 30 seconds ago
func recencyTier(t model.ToolEntry) int {
	if !t.Completed {
		return 0
	}
	if t.StartTime.IsZero() {
		return 2 // fallback for entries without timestamps
	}
	completedAt := t.StartTime.Add(time.Duration(t.DurationMs) * time.Millisecond)
	age := time.Since(completedAt)
	switch {
	case age < 5*time.Second:
		return 1
	case age < 30*time.Second:
		return 2
	default:
		return 3
	}
}

// toolLabel joins a category icon and tool name with a space so the icon
// has room to render at full cell size. Change the separator here to adjust
// the spacing between icon and name across all tool entries.
func toolLabel(icon, name string) string {
	return icon + " " + name
}

// renderToolEntryPlain formats a single tool entry as unstyled text.
func renderToolEntryPlain(icons Icons, t model.ToolEntry) string {
	label := toolLabel(CategoryIcon(icons, t.Category), t.Name)
	if !t.Completed {
		return label
	}
	return label + " " + formatDuration(t.DurationMs)
}

// renderToolEntry formats a single tool entry according to its state and recency.
//
// Styling by tier:
//   - Tier 0 (running): yellow icon + bold name (unchanged)
//   - Tier 1 (fresh, <5s): green icon + SecondaryStyle name + dim duration
//   - Tier 2 (recent, 5-30s): green icon + dim name + dim duration
//   - Tier 3 (faded, >30s): dim icon + dim name + dim duration
//   - Error: red icon + name + duration (regardless of age)
func renderToolEntry(icons Icons, t model.ToolEntry) string {
	catIcon := CategoryIcon(icons, t.Category)

	if !t.Completed {
		if t.Category == "Thinking" {
			return fmt.Sprintf("%s %s", yellowStyle.Render(catIcon), DimStyle.Render(t.Name))
		}
		return yellowStyle.Bold(true).Render(toolLabel(catIcon, t.Name))
	}

	if t.HasError {
		label := redStyle.Render(toolLabel(catIcon, t.Name))
		dur := redStyle.Render(formatDuration(t.DurationMs))
		return fmt.Sprintf("%s %s", label, dur)
	}

	tier := recencyTier(t)
	dur := DimStyle.Render(formatDuration(t.DurationMs))

	switch tier {
	case 1: // fresh (<5s): green icon + secondary (default fg) name
		icon := greenStyle.Render(catIcon)
		name := SecondaryStyle.Render(t.Name)
		return fmt.Sprintf("%s %s %s", icon, name, dur)
	case 3: // faded (>30s): dim icon + dim name
		icon := DimStyle.Render(catIcon)
		name := DimStyle.Render(t.Name)
		return fmt.Sprintf("%s %s %s", icon, name, dur)
	default: // tier 2 / recent: green icon + dim name
		icon := greenStyle.Render(catIcon)
		name := DimStyle.Render(t.Name)
		return fmt.Sprintf("%s %s %s", icon, name, dur)
	}
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
