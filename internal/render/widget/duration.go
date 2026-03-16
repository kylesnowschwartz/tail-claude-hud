package widget

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var durationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

// Duration renders the session elapsed time from ctx.SessionStart.
// ctx.SessionStart is an RFC3339 timestamp string representing when the session
// started. The widget computes elapsed time from that timestamp.
// Format: "Xh Ym" for sessions >= 1 hour, "Ym Xs" for shorter sessions.
// Returns "" when ctx.SessionStart is empty.
func Duration(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.SessionStart == "" {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)

	start, err := time.Parse(time.RFC3339, ctx.SessionStart)
	if err != nil {
		// SessionStart may already be a pre-formatted string — render as-is.
		return durationStyle.Render(fmt.Sprintf("%s%s", icons.Clock, ctx.SessionStart))
	}

	elapsed := time.Since(start)
	return durationStyle.Render(fmt.Sprintf("%s%s", icons.Clock, formatElapsed(elapsed)))
}

// formatElapsed formats a duration as "Xh Ym" or "Ym Xs".
func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return strings.TrimSpace(fmt.Sprintf("%dh %dm", h, m))
	}
	return strings.TrimSpace(fmt.Sprintf("%dm %ds", m, s))
}
