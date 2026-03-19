package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// agentSeparator is the text between agent entries.
const agentSeparator = " | "

// Agents renders running and recently-completed sub-agent entries.
// Running agents show a colored robot icon, half-circle indicator, and elapsed time.
// Completed agents show a dim colored robot icon, check mark, and duration.
// Returns an empty WidgetResult when ctx.Transcript is nil or there are no agents to show.
// FgColor is left empty because the widget composes multiple styles internally;
// the renderer passes the pre-styled Text through as-is.
func Agents(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	agents := ctx.Transcript.Agents

	var running []model.AgentEntry
	var completed []model.AgentEntry
	for _, a := range agents {
		if a.Status == "running" {
			running = append(running, a)
		} else if a.DurationMs >= 1000 && !isStaleAgent(a) {
			// Only show completed agents that ran for >= 1s and finished
			// within the last 60s. Older agents are no longer actionable.
			completed = append(completed, a)
		}
	}

	// Show all running agents + last 2 completed, max 5 total.
	recent := completed
	if len(recent) > 2 {
		recent = recent[len(recent)-2:]
	}
	toShow := append(running, recent...)
	if len(toShow) > 5 {
		toShow = toShow[len(toShow)-5:]
	}

	if len(toShow) == 0 {
		return WidgetResult{}
	}

	var parts []string
	var plainParts []string
	for _, a := range toShow {
		parts = append(parts, formatAgentEntry(a, icons))
		plainParts = append(plainParts, formatAgentEntryPlain(a, icons))
	}

	// When the terminal width is known, truncate trailing agents with a count
	// indicator (e.g. "+2 more") instead of letting the render stage cut mid-entry
	// with "...". This produces a more readable result when many agents are running.
	if ctx.TerminalWidth > 0 {
		parts, plainParts = truncateAgentEntries(parts, plainParts, ctx.TerminalWidth)
	}

	// Use the first agent's palette color as the dominant fg.
	fgColor := agentColors[toShow[0].ColorIndex%8]

	return WidgetResult{
		Text:      strings.Join(parts, agentSeparator),
		PlainText: strings.Join(plainParts, agentSeparator),
		FgColor:   fgColor,
	}
}

// truncateAgentEntries drops trailing entries that would push the joined output
// beyond maxWidth, appending a "+N more" indicator to signal the hidden count.
// It measures width using the plain-text variants (no ANSI codes) so that
// wide characters and multi-byte icons are counted correctly.
func truncateAgentEntries(styled, plain []string, maxWidth int) ([]string, []string) {
	// Rough overhead: ANSI reset prefix + reset suffix + erase-to-EOL added by
	// the renderer. We leave a conservative 8-column margin for those bytes.
	const rendererOverhead = 8

	available := maxWidth - rendererOverhead
	if available <= 0 {
		available = maxWidth
	}

	total := len(plain)
	for keep := total; keep > 0; keep-- {
		candidate := strings.Join(plain[:keep], agentSeparator)
		if keep < total {
			candidate += agentSeparator + fmt.Sprintf("+%d more", total-keep)
		}
		if ansi.StringWidth(candidate) <= available {
			if keep == total {
				// Everything fits — return as-is.
				return styled, plain
			}
			// Build new slices to avoid mutating the caller's backing arrays.
			suffix := fmt.Sprintf("+%d more", total-keep)
			outStyled := make([]string, keep+1)
			copy(outStyled, styled[:keep])
			outStyled[keep] = DimStyle.Render(suffix)
			outPlain := make([]string, keep+1)
			copy(outPlain, plain[:keep])
			outPlain[keep] = suffix
			return outStyled, outPlain
		}
	}

	// Fallback: not even one entry fits. Return just the first entry without
	// a count indicator so the render stage can truncate it further.
	return styled[:1], plain[:1]
}

// agentStaleThreshold is how long after completion before an agent entry
// is considered stale and hidden from the statusline.
const agentStaleThreshold = 60 * time.Second

// isStaleAgent reports whether a completed agent finished more than
// agentStaleThreshold ago. Returns false for running agents or agents
// without timing data.
func isStaleAgent(a model.AgentEntry) bool {
	if a.StartTime.IsZero() || a.DurationMs == 0 {
		return false
	}
	completedAt := a.StartTime.Add(time.Duration(a.DurationMs) * time.Millisecond)
	return time.Since(completedAt) > agentStaleThreshold
}

// maxAgentNameWidth is the character budget for the name/description portion
// of an agent entry. The full entry also includes icon, model suffix, status
// indicator, and elapsed/duration, so the name must be capped to prevent a
// single verbose description from consuming the entire line.
const maxAgentNameWidth = 25

// truncateAgentName caps name to maxWidth visible characters, appending "…"
// when truncation is needed. Returns name unchanged when it fits.
func truncateAgentName(name string, maxWidth int) string {
	if ansi.StringWidth(name) <= maxWidth {
		return name
	}
	return ansi.Truncate(name, maxWidth-1, "") + "…"
}

// formatAgentEntryPlain renders a single agent entry as unstyled text.
func formatAgentEntryPlain(a model.AgentEntry, icons Icons) string {
	displayName := a.Name
	if a.Description != "" {
		displayName = a.Description
	}
	displayName = truncateAgentName(displayName, maxAgentNameWidth)
	modelSuffix := modelFamilySuffix(a.Model)
	label := icons.Task + " " + displayName + modelSuffix

	if a.Status == "running" {
		elapsed := formatElapsed(time.Since(a.StartTime))
		return label + " " + icons.Running + " " + elapsed
	}
	return label + " " + icons.Check + " " + formatDuration(a.DurationMs)
}

// formatAgentEntry renders a single agent entry with colored icon, running
// indicator or check mark, and elapsed/duration time.
func formatAgentEntry(a model.AgentEntry, icons Icons) string {
	style := AgentColorStyle(a.ColorIndex)
	icon := icons.Task
	modelSuffix := modelFamilySuffix(a.Model)

	// Prefer the description ("Structural completeness review") over the
	// subagent_type ("general-purpose") when available. The description is
	// the human-readable task label from the Agent tool_use input.
	displayName := a.Name
	if a.Description != "" {
		displayName = a.Description
	}
	displayName = truncateAgentName(displayName, maxAgentNameWidth)

	if a.Status == "running" {
		elapsed := formatElapsed(time.Since(a.StartTime))
		label := icon + " " + displayName + modelSuffix
		return style.Render(label) + " " + style.Render(icons.Running) + " " + DimStyle.Render(elapsed)
	}

	// Completed: dim the colored icon, show check + duration.
	dimColorStyle := style.Faint(true)
	label := icon + " " + displayName + modelSuffix
	duration := formatDuration(a.DurationMs)
	return dimColorStyle.Render(label) + " " + greenStyle.Render(icons.Check) + DimStyle.Render(duration)
}

// modelFamilySuffix returns a parenthetical suffix for the model family if
// recognized, e.g. " (haiku)". Returns "" for unrecognized models.
func modelFamilySuffix(modelName string) string {
	family := ModelFamily(modelName)
	if family == "" {
		return ""
	}
	return " (" + family + ")"
}
