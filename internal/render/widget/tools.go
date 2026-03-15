package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	toolNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87")) // cyan
	toolDimStyle  = lipgloss.NewStyle().Faint(true)
)

// Tools renders running and recently-completed tool invocations.
// Running tools show a spinner icon; completed tools show a check icon with count.
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

	var parts []string

	// Separate running from completed based on Count sentinel:
	// Count == 0 means the tool is currently running; Count > 0 means completed.
	var running []model.ToolEntry
	var completed []model.ToolEntry
	for _, t := range tools {
		if t.Count == 0 {
			running = append(running, t)
		} else {
			completed = append(completed, t)
		}
	}

	// Show last two running tools.
	start := 0
	if len(running) > 2 {
		start = len(running) - 2
	}
	for _, t := range running[start:] {
		icon := yellowStyle.Render(icons.Spinner)
		name := toolNameStyle.Render(t.Name)
		parts = append(parts, fmt.Sprintf("%s %s", icon, name))
	}

	// Show up to four completed tools with counts.
	end := len(completed)
	if end > 4 {
		end = 4
	}
	for _, t := range completed[:end] {
		icon := greenStyle.Render(icons.Check)
		label := t.Name
		if t.Count > 1 {
			label = fmt.Sprintf("%s %s", t.Name, toolDimStyle.Render(fmt.Sprintf("x%d", t.Count)))
		}
		parts = append(parts, fmt.Sprintf("%s %s", icon, label))
	}

	return strings.Join(parts, " | ")
}
