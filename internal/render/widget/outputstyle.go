package widget

import (
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// OutputStyle renders the current Claude Code output style (e.g. "concise") with
// an edit/pen icon prefix and dim styling. Returns an empty WidgetResult when
// ctx.OutputStyle is empty.
func OutputStyle(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.OutputStyle == "" {
		return WidgetResult{}
	}
	icons := IconsFor(cfg.Style.Icons)
	plain := icons.Edit + " " + ctx.OutputStyle
	return WidgetResult{
		Text:      DimStyle.Render(plain),
		PlainText: plain,
		FgColor:   "8",
	}
}
