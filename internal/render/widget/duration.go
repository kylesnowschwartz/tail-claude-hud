package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var durationStyle = MutedStyle

// Duration renders the session elapsed time.
//
// Source priority:
//  1. ctx.TotalDurationMs (from Claude Code's stdin cost object) — authoritative,
//     measured by Claude Code itself and includes time before the HUD started.
//  2. ctx.SessionStart (RFC3339 timestamp derived from the transcript) — used as
//     a fallback when stdin duration is not available.
//
// Format: "Xh Ym" for sessions >= 1 hour, "Ym Xs" for shorter sessions.
// Returns an empty WidgetResult when neither source provides usable data.
func Duration(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	// Resolve the elapsed string from the best available source.
	var display string
	switch {
	case ctx.TotalDurationMs > 0:
		display = formatElapsed(time.Duration(ctx.TotalDurationMs) * time.Millisecond)
	case ctx.SessionStart != "":
		start, err := time.Parse(time.RFC3339, ctx.SessionStart)
		if err != nil {
			// SessionStart may already be a pre-formatted string — render as-is.
			display = ctx.SessionStart
		} else {
			display = formatElapsed(time.Since(start))
		}
	default:
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	plain := fmt.Sprintf("%s %s", icons.Clock, display)
	return WidgetResult{
		Text:      durationStyle.Render(plain),
		PlainText: plain,
		FgColor:   "8",
	}
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
