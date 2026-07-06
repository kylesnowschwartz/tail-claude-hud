package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Skills renders the most recently invoked skill name with a "+N more" suffix
// when multiple unique skills have been used. Plugin namespace prefixes
// (e.g. "sc-skills:") are stripped for display since the skill name alone
// is what matters on the status line.
//
// When cfg.Skills.MaxAgeMins > 0, invocations older than that are hidden so
// the widget reads as recent activity rather than a whole-session log; the
// line disappears entirely once every skill has gone stale. 0 shows all.
//
// Returns an empty WidgetResult when ctx.Transcript is nil or no skills
// remain after the age filter.
func Skills(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil || len(ctx.Transcript.Skills) == 0 {
		return WidgetResult{}
	}

	skills := ctx.Transcript.Skills
	if cfg.Skills.MaxAgeMins > 0 {
		cutoff := time.Now().Add(-time.Duration(cfg.Skills.MaxAgeMins) * time.Minute)
		fresh := make([]model.SkillInvocation, 0, len(skills))
		for _, s := range skills {
			if !s.Timestamp.Before(cutoff) {
				fresh = append(fresh, s)
			}
		}
		skills = fresh
	}
	if len(skills) == 0 {
		return WidgetResult{}
	}

	// Deduplicate while preserving most-recent-first order.
	seen := make(map[string]bool, len(skills))
	unique := make([]string, 0, len(skills))
	for i := len(skills) - 1; i >= 0; i-- {
		name := skills[i].Name
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
