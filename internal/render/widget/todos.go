package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Todos renders a completed/total count for the todo list.
// Color reflects completion ratio: green when all done, yellow when partial,
// dim when none are complete. Returns an empty WidgetResult when ctx.Transcript is nil or the
// todo list is empty.
// FgColor is left empty because the widget composes multiple styles internally;
// the renderer passes the pre-styled Text through as-is.
func Todos(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil {
		return WidgetResult{}
	}

	todos := ctx.Transcript.Todos
	if len(todos) == 0 {
		return WidgetResult{}
	}

	total := len(todos)
	done := 0
	for _, t := range todos {
		if t.Done {
			done++
		}
	}

	// All done — nothing left to act on. Hide the widget.
	if done == total {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	count := fmt.Sprintf("%d/%d", done, total)

	var text, plainIcon, fgColor string
	switch {
	case done > 0:
		// Partial — yellow spinner.
		text = fmt.Sprintf("%s %s", yellowStyle.Render(icons.Running), yellowStyle.Render(count))
		plainIcon = icons.Running
		fgColor = "3"
	default:
		// Nothing done yet — dim.
		text = fmt.Sprintf("%s %s", DimStyle.Render(icons.Running), DimStyle.Render(count))
		plainIcon = icons.Running
		fgColor = "8"
	}
	return WidgetResult{
		Text:      text,
		PlainText: plainIcon + " " + count,
		FgColor:   fgColor,
	}
}
