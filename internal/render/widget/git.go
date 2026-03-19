package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var gitBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

// Git renders branch name, dirty indicator, and optionally ahead/behind counts.
// Branch name is rendered in cyan. Dirty state uses the nerdfont dirty icon when
// cfg.Git.Dirty is true. Ahead/behind counts appear when cfg.Git.AheadBehind is true.
// Returns an empty WidgetResult when ctx.Git is nil.
// FgColor is left empty because the widget composes multiple styles internally;
// the renderer passes the pre-styled Text through as-is.
func Git(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Git == nil {
		return WidgetResult{}
	}

	icons := IconsFor(cfg.Style.Icons)
	g := ctx.Git

	var parts []string
	var plainParts []string

	// Branch icon + name in cyan.
	branchStr := fmt.Sprintf("%s%s", icons.Branch, g.Branch)
	parts = append(parts, gitBranchStyle.Render(branchStr))
	plainParts = append(plainParts, branchStr)

	// Dirty indicator (modified, staged, or untracked files).
	if cfg.Git.Dirty && g.IsDirty() {
		parts = append(parts, DimStyle.Render("*"))
		plainParts = append(plainParts, "*")
	}

	// Ahead/behind counts.
	if cfg.Git.AheadBehind {
		if g.AheadBy > 0 {
			ab := fmt.Sprintf("↑%d", g.AheadBy)
			parts = append(parts, DimStyle.Render(ab))
			plainParts = append(plainParts, ab)
		}
		if g.BehindBy > 0 {
			ab := fmt.Sprintf("↓%d", g.BehindBy)
			parts = append(parts, DimStyle.Render(ab))
			plainParts = append(plainParts, ab)
		}
	}

	// File stats: modified/staged/untracked counts.
	if cfg.Git.FileStats {
		var statParts []string
		var statPlainParts []string
		if g.Modified > 0 {
			s := fmt.Sprintf("~%d", g.Modified)
			statParts = append(statParts, yellowStyle.Render(s))
			statPlainParts = append(statPlainParts, s)
		}
		if g.Staged > 0 {
			s := fmt.Sprintf("+%d", g.Staged)
			statParts = append(statParts, greenStyle.Render(s))
			statPlainParts = append(statPlainParts, s)
		}
		if g.Untracked > 0 {
			s := fmt.Sprintf("?%d", g.Untracked)
			statParts = append(statParts, DimStyle.Render(s))
			statPlainParts = append(statPlainParts, s)
		}
		if len(statParts) > 0 {
			parts = append(parts, " "+strings.Join(statParts, ""))
			plainParts = append(plainParts, " "+strings.Join(statPlainParts, ""))
		}
	}

	return WidgetResult{
		Text:      strings.Join(parts, ""),
		PlainText: strings.Join(plainParts, ""),
		FgColor:   "14",
	}
}
