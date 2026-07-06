package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/color"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var gitBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

// Git renders branch name, dirty indicator, and optionally ahead/behind counts
// and uncommitted line deltas. Returns an empty WidgetResult when ctx.Git is nil.
//
// FgColor is empty per the composite-widget contract: the widget mixes
// semantic colors (green/red line deltas, dim decorators) that a theme fg
// override would flatten. The theme's git fg is honored here instead,
// applied to the branch portion only.
func Git(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Git == nil {
		return WidgetResult{}
	}

	text, plain := renderGitState(ctx.Git, cfg, themeFgStyle(cfg, "git", gitBranchStyle))
	return WidgetResult{
		Text:      text,
		PlainText: plain,
	}
}

// renderGitState renders the git segment body: branch (in branchStyle),
// dirty indicator, ahead/behind, line stats, and file stats. It is shared
// by the git widget and the project widget, which differ only in the base
// style applied to the branch.
func renderGitState(g *model.GitStatus, cfg *config.Config, branchStyle lipgloss.Style) (text, plain string) {
	icons := IconsFor(cfg.Style.Icons)

	var parts []string
	var plainParts []string

	// Branch icon + name in the caller's base style.
	branchStr := fmt.Sprintf("%s %s", icons.Branch, g.Branch)
	parts = append(parts, branchStyle.Render(branchStr))
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

	// Line stats: uncommitted +added -removed line deltas vs HEAD.
	// Styled to match the lines widget's "+N -M" convention.
	if cfg.Git.LineStats {
		if g.LinesAdded > 0 {
			s := fmt.Sprintf("+%d", g.LinesAdded)
			parts = append(parts, " "+linesAddedStyle.Render(s))
			plainParts = append(plainParts, " "+s)
		}
		if g.LinesRemoved > 0 {
			s := fmt.Sprintf("-%d", g.LinesRemoved)
			parts = append(parts, " "+linesRemovedStyle.Render(s))
			plainParts = append(plainParts, " "+s)
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

	return strings.Join(parts, ""), strings.Join(plainParts, "")
}

// themeFgStyle returns a fg-only style from the resolved theme's entry for
// widgetName, or fallback when the theme defines no fg. Composite widgets
// (FgColor == "") use this to honor the theme's base color on their identity
// text while keeping semantic colors on the rest.
func themeFgStyle(cfg *config.Config, widgetName string, fallback lipgloss.Style) lipgloss.Style {
	if cfg.ResolvedTheme != nil {
		if colors, ok := cfg.ResolvedTheme[widgetName]; ok && colors.Fg != "" {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(color.ResolveColorName(colors.Fg)))
		}
	}
	return fallback
}
