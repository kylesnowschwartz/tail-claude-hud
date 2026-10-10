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

	// Group consecutive entries with the same name so that e.g.
	// "Bash | Bash | Bash" becomes "Bash ×3". Each group counts as
	// one slot toward maxVisibleTools, letting more unique tool types
	// remain visible.
	groups := groupConsecutive(reversed)
	if len(groups) > maxVisibleTools {
		groups = groups[:maxVisibleTools]
	}

	var parts []string
	var plainParts []string
	for _, g := range groups {
		parts = append(parts, renderToolGroup(icons, g))
		plainParts = append(plainParts, renderToolGroupPlain(icons, g))
	}

	// Compute the highlighted separator position using wrapping ticker logic.
	// The highlight cycles through separator positions based on the number of
	// visible groups, so it walks one position per group-boundary change rather
	// than jumping multiple positions when consecutive tools are grouped.
	numSeps := len(parts) - 1
	if numSeps <= 0 {
		return WidgetResult{
			Text:      joinWithHighlight(parts, -1),
			PlainText: joinPlain(plainParts),
			FgColor:   "",
		}
	}
	highlightIdx := len(groups) % numSeps

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

// renderToolEntryPlain formats a tool entry as unstyled text.
// When count > 1, appends "×N" and omits the duration (ambiguous for groups).
func renderToolEntryPlain(icons Icons, t model.ToolEntry, count int) string {
	label := toolLabel(CategoryIcon(icons, t.Category), t.Name)
	if count > 1 {
		label += fmt.Sprintf(" ×%d", count)
		if !t.Completed {
			return label
		}
		return label
	}
	if !t.Completed {
		return label
	}
	return label + " " + formatDuration(t.DurationMs)
}

// renderToolEntry formats a tool entry according to its state and recency.
// When count > 1, appends a "×N" multiplier styled consistently with the name
// and omits the duration (ambiguous for groups).
//
// Styling by tier:
//   - Tier 0 (running): yellow icon + bold name (unchanged)
//   - Tier 1 (fresh, <5s): green icon + SecondaryStyle name + dim duration
//   - Tier 2 (recent, 5-30s): green icon + dim name + dim duration
//   - Tier 3 (faded, >30s): dim icon + dim name + dim duration
//   - Error: red icon + name + duration (regardless of age)
func renderToolEntry(icons Icons, t model.ToolEntry, count int) string {
	catIcon := CategoryIcon(icons, t.Category)
	mult := ""
	if count > 1 {
		mult = fmt.Sprintf(" ×%d", count)
	}
	showDuration := count <= 1

	if !t.Completed {
		if t.Category == "Thinking" {
			return fmt.Sprintf("%s %s", yellowStyle.Render(catIcon), DimStyle.Render(t.Name+mult))
		}
		return yellowStyle.Bold(true).Render(toolLabel(catIcon, t.Name) + mult)
	}

	if t.HasError {
		label := redStyle.Render(toolLabel(catIcon, t.Name) + mult)
		if !showDuration {
			return label
		}
		dur := redStyle.Render(formatDuration(t.DurationMs))
		return fmt.Sprintf("%s %s", label, dur)
	}

	tier := recencyTier(t)

	switch tier {
	case 1: // fresh (<5s): green icon + secondary (default fg) name
		icon := greenStyle.Render(catIcon)
		name := SecondaryStyle.Render(t.Name + mult)
		if !showDuration {
			return fmt.Sprintf("%s %s", icon, name)
		}
		return fmt.Sprintf("%s %s %s", icon, name, DimStyle.Render(formatDuration(t.DurationMs)))
	case 3: // faded (>30s): dim icon + dim name
		icon := DimStyle.Render(catIcon)
		name := DimStyle.Render(t.Name + mult)
		if !showDuration {
			return fmt.Sprintf("%s %s", icon, name)
		}
		return fmt.Sprintf("%s %s %s", icon, name, DimStyle.Render(formatDuration(t.DurationMs)))
	default: // tier 2 / recent: green icon + dim name
		icon := greenStyle.Render(catIcon)
		name := DimStyle.Render(t.Name + mult)
		if !showDuration {
			return fmt.Sprintf("%s %s", icon, name)
		}
		return fmt.Sprintf("%s %s %s", icon, name, DimStyle.Render(formatDuration(t.DurationMs)))
	}
}

// toolGroup represents one or more consecutive tool entries with the same name.
// The representative entry (the first / most recent) is used for styling.
type toolGroup struct {
	Entry model.ToolEntry // most recent entry in the group (for styling)
	Count int             // number of consecutive entries collapsed
}

// groupConsecutive collapses runs of consecutive entries with the same name
// into toolGroup values. A single entry becomes a group with Count=1.
func groupConsecutive(entries []model.ToolEntry) []toolGroup {
	if len(entries) == 0 {
		return nil
	}
	groups := []toolGroup{{Entry: entries[0], Count: 1}}
	for _, e := range entries[1:] {
		last := &groups[len(groups)-1]
		if e.Name == last.Entry.Name {
			last.Count++
		} else {
			groups = append(groups, toolGroup{Entry: e, Count: 1})
		}
	}
	return groups
}

// renderToolGroup renders a toolGroup, delegating to renderToolEntry with the
// group's count. Count=1 renders normally; count>1 appends ×N and hides duration.
func renderToolGroup(icons Icons, g toolGroup) string {
	return renderToolEntry(icons, g.Entry, g.Count)
}

// renderToolGroupPlain renders a toolGroup as unstyled text.
func renderToolGroupPlain(icons Icons, g toolGroup) string {
	return renderToolEntryPlain(icons, g.Entry, g.Count)
}

// formatDuration converts a millisecond duration into the compact string every
// widget uses for elapsed time.
//
//   - <= 0ms:             "0.0s"  (genuinely instant or unknown)
//   - 1ms – 99ms:        "<0.1s" (sub-100ms; avoids misleading "0.0s" for real durations)
//   - 100ms – 999ms:     "0.Xs"  (tenths of a second)
//   - 1s – 59.9s:        "Xs" or "X.Ys" (seconds, optional tenth)
//   - 1m – 59m 59s:      "Xm Ys"
//   - >= 1h:             "Xh Ym"
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
	if ms < msPerMinute {
		secs := ms / 1000
		frac := (ms % 1000) / 100
		if frac == 0 {
			return fmt.Sprintf("%ds", secs)
		}
		return fmt.Sprintf("%d.%ds", secs, frac)
	}
	if ms < msPerHour {
		return fmt.Sprintf("%dm %ds", ms/msPerMinute, (ms%msPerMinute)/1000)
	}
	return fmt.Sprintf("%dh %dm", ms/msPerHour, (ms%msPerHour)/msPerMinute)
}

const (
	msPerMinute = 60 * 1000
	msPerHour   = 60 * msPerMinute
)

// formatElapsed formats a live clock (session time, a running agent) in the
// formatDuration shape at whole-second resolution, so the tenths digit does
// not flicker between ticks.
func formatElapsed(d time.Duration) string {
	return formatDuration(int(d.Round(time.Second).Milliseconds()))
}
