package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	usageBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // bright blue default
	usageDimStyle = lipgloss.NewStyle().Faint(true)
)

// Usage renders an API context usage bar and percentage from ctx.Usage.
// Bar color is bright blue by default; the bar width matches cfg.Context.BarWidth.
// Returns "" when ctx.Usage is nil.
func Usage(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Usage == nil {
		return ""
	}

	pct := ctx.Usage.ContextPercent
	if pct == 0 && ctx.Usage.ContextWindowSize == 0 {
		return ""
	}

	barWidth := cfg.Context.BarWidth
	if barWidth <= 0 {
		barWidth = 10
	}

	filled := (pct * barWidth) / 100
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	bar := usageBarStyle.Render(strings.Repeat("█", filled)) +
		usageDimStyle.Render(strings.Repeat("░", empty))

	return bar + " " + usageBarStyle.Render(fmt.Sprintf("%d%%", pct))
}
