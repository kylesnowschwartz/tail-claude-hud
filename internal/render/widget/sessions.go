package widget

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	sessionsRunningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	sessionsIdleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
)

// Sessions renders other discovered Claude Code sessions as a row of dots.
// Running sessions show a filled green dot (●), idle sessions show an open
// dim dot (○). Each dot is optionally followed by the project name when
// config.Sessions.ShowProject is true.
//
// Returns an empty WidgetResult when no other sessions are found.
func Sessions(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if len(ctx.Sessions) == 0 {
		return WidgetResult{}
	}

	var styledParts []string
	var plainParts []string

	for _, s := range ctx.Sessions {
		var dot, label string
		var style lipgloss.Style

		if s.Running {
			dot = "\u25CF" // ● filled circle
			style = sessionsRunningStyle
		} else {
			dot = "\u25CB" // ○ open circle
			style = sessionsIdleStyle
		}

		if cfg.Sessions.ShowProject && s.Project != "" {
			label = dot + " " + s.Project
		} else {
			label = dot
		}

		styledParts = append(styledParts, style.Render(label))
		plainParts = append(plainParts, label)
	}

	return WidgetResult{
		Text:      strings.Join(styledParts, "  "),
		PlainText: strings.Join(plainParts, "  "),
		FgColor:   "2", // green (dominant color for powerline/minimal)
	}
}
