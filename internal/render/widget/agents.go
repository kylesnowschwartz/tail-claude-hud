package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Agents renders running and recently-completed sub-agent entries.
// Running agents show a colored robot icon, half-circle indicator, and elapsed time.
// Completed agents show a dim colored robot icon, check mark, and duration.
// Returns "" when ctx.Transcript is nil or there are no agents to show.
func Agents(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Transcript == nil {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)
	agents := ctx.Transcript.Agents

	var running []model.AgentEntry
	var completed []model.AgentEntry
	for _, a := range agents {
		if a.Status == "running" {
			running = append(running, a)
		} else if a.DurationMs >= 1000 {
			// Only show completed agents that ran for >= 1s.
			// Sub-second agents are noise — they completed before
			// the user could notice them.
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
		return ""
	}

	var parts []string
	for _, a := range toShow {
		parts = append(parts, formatAgentEntry(a, icons))
	}

	return strings.Join(parts, " | ")
}

// formatAgentEntry renders a single agent entry with colored icon, running
// indicator or check mark, and elapsed/duration time.
func formatAgentEntry(a model.AgentEntry, icons Icons) string {
	style := AgentColorStyle(a.ColorIndex)
	icon := icons.Agent
	modelSuffix := modelFamilySuffix(a.Model)

	if a.Status == "running" {
		elapsed := formatElapsed(time.Since(a.StartTime))
		label := icon + " " + a.Name + modelSuffix
		return style.Render(label) + " " + style.Render(icons.Running) + " " + dimStyle.Render(elapsed)
	}

	// Completed: dim the colored icon, show check + duration.
	dimColorStyle := style.Faint(true)
	label := icon + " " + a.Name + modelSuffix
	duration := formatDurationMs(a.DurationMs)
	return dimColorStyle.Render(label) + " " + greenStyle.Render(icons.Check) + " " + dimStyle.Render(duration)
}

// modelFamilySuffix returns a dim parenthetical suffix for the model family if
// the model string is non-empty, e.g. " (haiku)". Returns "" when unrecognized.
func modelFamilySuffix(model string) string {
	lower := strings.ToLower(model)
	var family string
	switch {
	case strings.Contains(lower, "opus"):
		family = "opus"
	case strings.Contains(lower, "sonnet"):
		family = "sonnet"
	case strings.Contains(lower, "haiku"):
		family = "haiku"
	}
	if family == "" {
		return ""
	}
	return " (" + family + ")"
}


// formatDurationMs formats a millisecond duration as a compact human-readable string:
//   - <1000ms  → "0.Xs"
//   - 1–60s    → "Xs"
//   - >60s     → "Xm Ys"
func formatDurationMs(ms int) string {
	if ms < 1000 {
		tenths := ms / 100
		return fmt.Sprintf("0.%ds", tenths)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%dm%ds", minutes, secs)
}

