package widget

import (
	"fmt"
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
// Format: formatElapsed ("45s", "1m 12s", "1h 3m").
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
