package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var envStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

// Env renders a summary of the active environment: MCP servers and allowed tools.
// Uses nerdfont icons when configured; falls back to unicode or ascii.
// Returns "" when ctx.EnvCounts is nil.
func Env(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.EnvCounts == nil {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)
	var parts []string

	if ctx.EnvCounts.MCPServers > 0 {
		parts = append(parts, fmt.Sprintf("%s%d", icons.Spinner, ctx.EnvCounts.MCPServers))
	}

	if ctx.EnvCounts.ToolsAllowed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d", icons.Check, ctx.EnvCounts.ToolsAllowed))
	}

	if len(parts) == 0 {
		return ""
	}

	return envStyle.Render(strings.Join(parts, " "))
}
