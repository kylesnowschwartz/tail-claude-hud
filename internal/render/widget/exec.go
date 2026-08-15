package widget

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Exec renders the output of the user-configured [exec] command as a widget
// segment. The command owns its own styling: Text passes the sanitized output
// through verbatim (SGR colors intact) for plain mode, while PlainText strips
// ANSI for powerline/minimal modes, which apply theme colors themselves.
// Returns an empty WidgetResult when no command is configured or it produced
// no output.
func Exec(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.ExecOutput == "" {
		return WidgetResult{}
	}

	return WidgetResult{
		Text:      ctx.ExecOutput,
		PlainText: ansi.Strip(ctx.ExecOutput),
	}
}
