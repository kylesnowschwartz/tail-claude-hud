package widget

import (
	"strings"
	"testing"

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
			SkillNames: []string{"commit"},
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
			SkillNames: []string{"commit", "review-pr", "lint"},
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
			SkillNames: []string{"commit", "lint", "commit"},
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
			SkillNames: []string{"sc-skills:effective-go"},
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
