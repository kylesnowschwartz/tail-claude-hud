package widget

import (
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Worktree renders the current worktree name when running inside a git worktree.
// Returns an empty WidgetResult when not in a worktree.
func Worktree(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.WorktreeName == "" {
		return WidgetResult{}
	}
	icons := IconsFor(cfg.Style.Icons)
	plain := icons.Branch + " wt:" + ctx.WorktreeName
	return WidgetResult{
		Text:      yellowStyle.Render(plain),
		PlainText: plain,
		FgColor:   "3",
	}
}
