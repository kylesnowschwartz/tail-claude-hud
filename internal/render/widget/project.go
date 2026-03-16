package widget

import (
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Project renders a merged directory + git segment.
// Format: '{directory} {branch}{dirty}{ahead}{behind}'
// e.g. 'tail-claude-hud main*' or 'tail-claude-hud feat/auth↑2'
// Directory is magenta bold; branch name is cyan; dirty/ahead/behind are dim.
// Returns "" when ctx.Cwd is empty.
// When ctx.Git is nil, renders directory only (no git suffix).
func Project(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Cwd == "" {
		return ""
	}

	levels := cfg.Directory.Levels
	if levels <= 0 {
		levels = 1
	}

	dirName := lastNSegments(ctx.Cwd, levels)
	dir := dirStyle.Render(dirName)

	if ctx.Git == nil {
		return dir
	}

	g := ctx.Git
	branch := gitBranchStyle.Render(g.Branch)

	// Build the dim suffix: dirty indicator, ahead, behind.
	var dimParts strings.Builder
	if g.IsDirty() {
		dimParts.WriteString("*")
	}
	if g.AheadBy > 0 {
		dimParts.WriteString(fmt.Sprintf("↑%d", g.AheadBy))
	}
	if g.BehindBy > 0 {
		dimParts.WriteString(fmt.Sprintf("↓%d", g.BehindBy))
	}

	suffix := dimParts.String()
	if suffix != "" {
		return dir + " " + branch + gitDimStyle.Render(suffix)
	}
	return dir + " " + branch
}
