package widget

import (
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

	// Prefer the description ("Structural completeness review") over the
	// subagent_type ("general-purpose") when available. The description is
	// the human-readable task label from the Agent tool_use input.
	displayName := a.Name
	if a.Description != "" {
		displayName = a.Description
	}

	if a.Status == "running" {
		elapsed := formatElapsed(time.Since(a.StartTime))
		label := icon + " " + displayName + modelSuffix
		return style.Render(label) + " " + style.Render(icons.Running) + " " + dimStyle.Render(elapsed)
	}

	// Completed: dim the colored icon, show check + duration.
	dimColorStyle := style.Faint(true)
	label := icon + " " + displayName + modelSuffix
	duration := formatDuration(a.DurationMs)
	return dimColorStyle.Render(label) + " " + greenStyle.Render(icons.Check) + " " + dimStyle.Render(duration)
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
