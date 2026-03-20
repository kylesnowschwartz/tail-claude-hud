package widget

import (
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Skills renders the most recently invoked skill name with a "+N more" suffix
// when multiple unique skills have been used. Plugin namespace prefixes
// (e.g. "sc-skills:") are stripped for display since the skill name alone
// is what matters on the status line.
//
// Returns an empty WidgetResult when ctx.Transcript is nil or no skills
// have been invoked.
func Skills(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil || len(ctx.Transcript.SkillNames) == 0 {
		return WidgetResult{}
	}

	// Deduplicate while preserving most-recent-first order.
	seen := make(map[string]bool, len(ctx.Transcript.SkillNames))
	unique := make([]string, 0, len(ctx.Transcript.SkillNames))
	for i := len(ctx.Transcript.SkillNames) - 1; i >= 0; i-- {
		name := ctx.Transcript.SkillNames[i]
		if !seen[name] {
			seen[name] = true
			unique = append(unique, name)
		}
	}

	icons := IconsFor(cfg.Style.Icons)
	label := icons.Skill + " " + shortSkillName(unique[0])
	if len(unique) > 1 {
		label += fmt.Sprintf(" +%d more", len(unique)-1)
	}

	return WidgetResult{
		Text:      MutedStyle.Render(label),
		PlainText: label,
		FgColor:   "8",
	}
}

// shortSkillName strips the plugin namespace prefix from a skill name.
// "sc-skills:effective-go" becomes "effective-go"; "commit" stays "commit".
func shortSkillName(name string) string {
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return name
}
