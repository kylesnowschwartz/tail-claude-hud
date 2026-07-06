package widget

import (
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Project composes the Directory and Git widgets into a single segment
// without a separator between them.
// Format: '{directory} {branch}{dirty}{ahead}{behind}'
// e.g. 'tail-claude-hud main*' or 'tail-claude-hud feat/auth↑2'
// Returns an empty WidgetResult when both sub-widgets are empty.
// When Git has no data, renders directory only.
// When in a worktree, the git branch is hidden (the worktree widget shows
// "wt:<branch>"), so only the project directory name is rendered.
//
// FgColor is empty per the composite-widget contract: the git portion mixes
// semantic colors (green/yellow file stats, dim decorators) that a theme fg
// override would flatten. The theme's project fg is honored here instead,
// applied uniformly to the directory and branch identity text.
func Project(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	dir := Directory(ctx, cfg)
	if dir.IsEmpty() {
		return WidgetResult{}
	}

	base := themeFgStyle(cfg, "project", dirStyle)
	dirOnly := WidgetResult{
		Text:      base.Render(dir.PlainText),
		PlainText: dir.PlainText,
	}

	// In a worktree the branch is shown by the worktree widget — just show
	// the project directory so the user keeps their project identity.
	if ctx.WorktreeName != "" || ctx.Git == nil {
		return dirOnly
	}

	gitText, gitPlain := renderGitState(ctx.Git, cfg, base)
	return WidgetResult{
		Text:      dirOnly.Text + " " + gitText,
		PlainText: dirOnly.PlainText + " " + gitPlain,
	}
}
