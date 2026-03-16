package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Todos renders a completed/total count for the todo list.
// Color reflects completion ratio: green when all done, yellow when partial,
// dim when none are complete. Returns "" when ctx.Transcript is nil or the
// todo list is empty.
func Todos(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Transcript == nil {
		return ""
	}

	todos := ctx.Transcript.Todos
	if len(todos) == 0 {
		return ""
	}

	total := len(todos)
	done := 0
	for _, t := range todos {
		if t.Done {
			done++
		}
	}

	icons := IconsFor(cfg.Style.Icons)
	count := fmt.Sprintf("%d/%d", done, total)

	switch {
	case done == total:
		// All complete — green check.
		return fmt.Sprintf("%s %s", greenStyle.Render(icons.Check), greenStyle.Render(count))
	case done > 0:
		// Partial — yellow spinner.
		return fmt.Sprintf("%s %s", yellowStyle.Render(icons.Running), yellowStyle.Render(count))
	default:
		// Nothing done yet — dim.
		return fmt.Sprintf("%s %s", dimStyle.Render(icons.Running), dimStyle.Render(count))
	}
}
