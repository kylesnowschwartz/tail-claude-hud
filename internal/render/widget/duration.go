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

// Duration renders the session elapsed time.
//
// Source priority:
//  1. ctx.TotalDurationMs (from Claude Code's stdin cost object) — authoritative,
//     measured by Claude Code itself and includes time before the HUD started.
//  2. ctx.SessionStart (RFC3339 timestamp derived from the transcript) — used as
//     a fallback when stdin duration is not available.
//
// Format: "Xh Ym" for sessions >= 1 hour, "Ym Xs" for shorter sessions.
// Returns "" when neither source provides usable data.
func Duration(ctx *model.RenderContext, cfg *config.Config) string {
	icons := IconsFor(cfg.Style.Icons)

	// Prefer the authoritative duration from stdin when available.
	if ctx.TotalDurationMs > 0 {
		elapsed := time.Duration(ctx.TotalDurationMs) * time.Millisecond
		return durationStyle.Render(fmt.Sprintf("%s%s", icons.Clock, formatElapsed(elapsed)))
	}

	// Fall back to transcript-derived start time.
	if ctx.SessionStart == "" {
		return ""
	}

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
