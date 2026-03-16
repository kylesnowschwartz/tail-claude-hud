package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	gitBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87"))
	gitDimStyle    = lipgloss.NewStyle().Faint(true)
)

// Git renders branch name, dirty indicator, and optionally ahead/behind counts.
// Branch name is rendered in cyan. Dirty state uses the nerdfont dirty icon when
// cfg.Git.Dirty is true. Ahead/behind counts appear when cfg.Git.AheadBehind is true.
// Returns "" when ctx.Git is nil.
func Git(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Git == nil {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)
	g := ctx.Git

	var parts []string

	// Branch icon + name in cyan.
	branch := gitBranchStyle.Render(fmt.Sprintf("%s%s", icons.Branch, g.Branch))
	parts = append(parts, branch)

	// Dirty indicator (modified, staged, or untracked files).
	if cfg.Git.Dirty && g.IsDirty() {
		parts = append(parts, gitDimStyle.Render("*"))
	}

	// Ahead/behind counts.
	if cfg.Git.AheadBehind {
		if g.AheadBy > 0 {
			parts = append(parts, gitDimStyle.Render(fmt.Sprintf("+%d", g.AheadBy)))
		}
		if g.BehindBy > 0 {
			parts = append(parts, gitDimStyle.Render(fmt.Sprintf("-%d", g.BehindBy)))
		}
	}

	return strings.Join(parts, "")
}
