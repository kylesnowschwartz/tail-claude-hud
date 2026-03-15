package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	agentNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // magenta
)

// Agents renders running and recently-completed sub-agent entries.
// Each entry shows a status icon, agent name, and status label.
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
		} else {
			completed = append(completed, a)
		}
	}

	// Show all running agents plus up to two recently completed.
	recent := completed
	if len(recent) > 2 {
		recent = recent[len(recent)-2:]
	}
	toShow := append(running, recent...)
	if len(toShow) > 3 {
		toShow = toShow[len(toShow)-3:]
	}

	if len(toShow) == 0 {
		return ""
	}

	var parts []string
	for _, a := range toShow {
		parts = append(parts, formatAgent(a, icons))
	}

	return strings.Join(parts, " | ")
}

func formatAgent(a model.AgentEntry, icons Icons) string {
	var icon string
	if a.Status == "running" {
		icon = yellowStyle.Render(icons.Spinner)
	} else {
		icon = greenStyle.Render(icons.Check)
	}

	name := agentNameStyle.Render(a.Name)
	status := dimStyle.Render(fmt.Sprintf("[%s]", a.Status))

	return fmt.Sprintf("%s %s %s", icon, name, status)
}
