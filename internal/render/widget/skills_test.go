package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestSkillsWidget_NilTranscript_ReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Skills(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty for nil transcript, got %q", got.Text)
	}
}

func TestSkillsWidget_NoSkills_ReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{},
	}
	cfg := defaultCfg()

	if got := Skills(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty when no skills, got %q", got.Text)
	}
}

func TestSkillsWidget_SingleSkill_DisplaysName(t *testing.T) {
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: skillList("commit"),
		},
	}
	cfg := defaultCfg()

	got := Skills(ctx, cfg)
	if !strings.Contains(got.PlainText, "commit") {
		t.Errorf("expected output to contain 'commit', got %q", got.PlainText)
	}
	// Single skill should not show "+N more".
	if strings.Contains(got.PlainText, "more") {
		t.Errorf("single skill should not show '+N more', got %q", got.PlainText)
	}
}

func TestSkillsWidget_MultipleSkills_ShowsNewestPlusCount(t *testing.T) {
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: skillList("commit", "review-pr", "lint"),
		},
	}
	cfg := defaultCfg()

	got := Skills(ctx, cfg)
	// "lint" is the newest (last in slice), should be the displayed name.
	if !strings.Contains(got.PlainText, "lint") {
		t.Errorf("expected newest skill 'lint' in output, got %q", got.PlainText)
	}
	// Should show "+2 more" for the other two unique skills.
	if !strings.Contains(got.PlainText, "+2 more") {
		t.Errorf("expected '+2 more' suffix, got %q", got.PlainText)
	}
}

func TestSkillsWidget_DuplicateSkills_DeduplicatesBeforeCounting(t *testing.T) {
	// "commit" appears twice; after dedup there are 2 unique skills.
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: skillList("commit", "lint", "commit"),
		},
	}
	cfg := defaultCfg()

	got := Skills(ctx, cfg)
	// "commit" is the newest (last), should be displayed.
	if !strings.Contains(got.PlainText, "commit") {
		t.Errorf("expected newest skill 'commit' in output, got %q", got.PlainText)
	}
	// 2 unique skills, so "+1 more".
	if !strings.Contains(got.PlainText, "+1 more") {
		t.Errorf("expected '+1 more' for 2 unique skills, got %q", got.PlainText)
	}
}

func TestSkillsWidget_NamespacedSkill_StripsPrefix(t *testing.T) {
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: skillList("sc-skills:effective-go"),
		},
	}
	cfg := defaultCfg()

	got := Skills(ctx, cfg)
	if !strings.Contains(got.PlainText, "effective-go") {
		t.Errorf("expected stripped name 'effective-go', got %q", got.PlainText)
	}
	if strings.Contains(got.PlainText, "sc-skills:") {
		t.Errorf("namespace prefix should be stripped, got %q", got.PlainText)
	}
}

func TestShortSkillName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"commit", "commit"},
		{"sc-skills:effective-go", "effective-go"},
		{"my-plugin:deploy", "deploy"},
		{"a:b:c", "c"},
	}
	for _, tt := range tests {
		if got := shortSkillName(tt.input); got != tt.want {
			t.Errorf("shortSkillName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// skillList builds fresh (now-stamped) invocations from bare names, letting
// the pre-timestamp assertions above stay written against name lists.
func skillList(names ...string) []model.SkillInvocation {
	skills := make([]model.SkillInvocation, 0, len(names))
	for _, n := range names {
		skills = append(skills, model.SkillInvocation{Name: n, Timestamp: time.Now()})
	}
	return skills
}

// TestSkillsWidget_MaxAge_HidesStaleInvocations pins the recency fade: with
// max_age_mins set, invocations older than the cutoff are hidden, and the
// widget goes empty once every skill is stale (the session log stays intact
// upstream — this is display-only filtering).
func TestSkillsWidget_MaxAge_HidesStaleInvocations(t *testing.T) {
	now := time.Now()
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: []model.SkillInvocation{
				{Name: "plugin-dev", Timestamp: now.Add(-2 * time.Hour)},
				{Name: "commit", Timestamp: now.Add(-3 * time.Minute)},
			},
		},
	}
	cfg := defaultCfg()
	cfg.Skills.MaxAgeMins = 15

	got := Skills(ctx, cfg)
	if !strings.Contains(got.PlainText, "commit") {
		t.Errorf("expected fresh skill 'commit' in output, got %q", got.PlainText)
	}
	if strings.Contains(got.PlainText, "plugin-dev") || strings.Contains(got.PlainText, "more") {
		t.Errorf("stale skill leaked into output: %q", got.PlainText)
	}

	// All stale → widget hides entirely.
	ctx.Transcript.Skills = ctx.Transcript.Skills[:1]
	if got := Skills(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty when all skills are stale, got %q", got.PlainText)
	}
}

// TestSkillsWidget_MaxAgeZero_ShowsFullSessionLog pins the default: 0 keeps
// every invocation visible regardless of age.
func TestSkillsWidget_MaxAgeZero_ShowsFullSessionLog(t *testing.T) {
	ctx := &model.RenderContext{
		Transcript: &model.TranscriptData{
			Skills: []model.SkillInvocation{
				{Name: "plugin-dev", Timestamp: time.Now().Add(-6 * time.Hour)},
			},
		},
	}
	cfg := defaultCfg()
	cfg.Skills.MaxAgeMins = 0

	if got := Skills(ctx, cfg); !strings.Contains(got.PlainText, "plugin-dev") {
		t.Errorf("expected old skill shown with max_age_mins=0, got %q", got.PlainText)
	}
}
