package widget

import (
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var permissionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

// Permission renders a red alert when another Claude Code session is waiting
// for permission approval. Returns an empty WidgetResult when no session needs
// attention, so the widget occupies zero space in normal operation.
//
// When config.Permission.ShowProject is true (default), the project name of the
// waiting session is displayed next to the icon (e.g. " my-project").
// When false, only the icon is shown.
func Permission(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.PermissionProject == "" {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	icon := icons.Permission

	label := icon
	if cfg.Permission.ShowProject {
		label = icon + " " + ctx.PermissionProject
	}

	return WidgetResult{
		Text:      permissionStyle.Render(label),
		PlainText: label,
		FgColor:   "1", // red
	}
}
