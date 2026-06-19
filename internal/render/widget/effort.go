package widget

import (
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// effortFgColor maps a reasoning effort level to an ANSI color string. Higher
// effort shifts warmer (yellow→red) to signal increased cost/latency; lower
// effort stays muted. Unrecognized levels fall back to the terminal default.
func effortFgColor(level string) string {
	switch level {
	case "low":
		return "8" // muted
	case "medium":
		return "6" // cyan
	case "high":
		return "3" // yellow
	case "xhigh", "max":
		return "1" // red
	default:
		return "7"
	}
}

// Effort renders the current reasoning effort level (low/medium/high/xhigh/max)
// with a lightbulb icon. Returns an empty WidgetResult when Claude Code does not
// report an effort level, so the widget occupies zero space when unavailable.
func Effort(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.EffortLevel == "" {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	plain := icons.Thinking + " " + ctx.EffortLevel
	fg := effortFgColor(ctx.EffortLevel)

	return WidgetResult{
		Text:      lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Render(plain),
		PlainText: plain,
		FgColor:   fg,
	}
}
