package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Peers renders a compact "×N" count of other concurrently running Claude
// Code sessions, detected via active-session heartbeats (see
// internal/breadcrumb). Returns empty when there are none, so the widget
// occupies zero space in the common single-session case.
func Peers(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.PeerCount <= 0 {
		return WidgetResult{}
	}

	text := fmt.Sprintf("×%d", ctx.PeerCount)
	return WidgetResult{
		Text:      DimStyle.Render(text),
		PlainText: text,
	}
}
