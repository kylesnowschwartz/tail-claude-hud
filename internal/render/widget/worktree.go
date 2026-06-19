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
	// Append the branch when configured and it differs from the worktree name
	// (showing "wt:foo foo" would be redundant).
	if cfg.Worktree.ShowBranch && ctx.WorktreeBranch != "" && ctx.WorktreeBranch != ctx.WorktreeName {
		plain += " " + ctx.WorktreeBranch
	}
	return WidgetResult{
		Text:      yellowStyle.Render(plain),
		PlainText: plain,
		FgColor:   "3",
	}
}
