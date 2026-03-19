package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Thinking renders a peripheral signal for active or completed thinking blocks.
//
// When ThinkingActive is true it shows the thinking icon in yellow — a live
// signal that Claude is currently reasoning. When thinking has completed
// (ThinkingCount > 0 but not active) it shows the icon in dim with the total
// count, giving a quick audit trail. Returns an empty WidgetResult when no thinking has occurred.
// FgColor is left empty because the widget selects between two different styles
// (yellow vs dim) based on state; the renderer passes the pre-styled Text through as-is.
func Thinking(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)

	if ctx.Transcript.ThinkingActive {
		return WidgetResult{
			Text:      yellowStyle.Render(icons.Thinking),
			PlainText: icons.Thinking,
			FgColor:   "3",
		}
	}

	if ctx.Transcript.ThinkingCount > 0 {
		plain := fmt.Sprintf("%s%d", icons.Thinking, ctx.Transcript.ThinkingCount)
		return WidgetResult{
			Text:      DimStyle.Render(plain),
			PlainText: plain,
			FgColor:   "8",
		}
	}

	return WidgetResult{}
}
