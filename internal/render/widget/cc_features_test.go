package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestEffortWidget_EmptyWhenUnset(t *testing.T) {
	cfg := defaultCfg()
	if got := Effort(&model.RenderContext{}, cfg); !got.IsEmpty() {
		t.Errorf("expected empty result for empty effort, got %q", got.Text)
	}
}

func TestEffortWidget_ShowsLevel(t *testing.T) {
	cfg := defaultCfg()
	got := Effort(&model.RenderContext{EffortLevel: "high"}, cfg)
	if !strings.Contains(got.PlainText, "high") {
		t.Errorf("expected effort level in output, got %q", got.PlainText)
	}
}

func TestEffortWidget_ColorByIntensity(t *testing.T) {
	cases := map[string]string{
		"low":    "8",
		"medium": "6",
		"high":   "3",
		"xhigh":  "1",
		"max":    "1",
	}
	cfg := defaultCfg()
	for level, want := range cases {
		got := Effort(&model.RenderContext{EffortLevel: level}, cfg)
		if got.FgColor != want {
			t.Errorf("effort %q: FgColor = %q, want %q", level, got.FgColor, want)
		}
	}
}

func TestWorktreeWidget_AppendsBranch(t *testing.T) {
	cfg := defaultCfg()
	cfg.Worktree.ShowBranch = true
	got := Worktree(&model.RenderContext{WorktreeName: "fix-bug", WorktreeBranch: "feat/cache"}, cfg)
	if !strings.Contains(got.PlainText, "fix-bug") || !strings.Contains(got.PlainText, "feat/cache") {
		t.Errorf("expected name and branch, got %q", got.PlainText)
	}
}

func TestWorktreeWidget_OmitsBranchWhenDisabled(t *testing.T) {
	cfg := defaultCfg()
	cfg.Worktree.ShowBranch = false
	got := Worktree(&model.RenderContext{WorktreeName: "fix-bug", WorktreeBranch: "feat/cache"}, cfg)
	if strings.Contains(got.PlainText, "feat/cache") {
		t.Errorf("expected branch omitted, got %q", got.PlainText)
	}
}

func TestWorktreeWidget_OmitsRedundantBranch(t *testing.T) {
	cfg := defaultCfg()
	cfg.Worktree.ShowBranch = true
	// Branch identical to name should not be duplicated ("wt:foo foo").
	got := Worktree(&model.RenderContext{WorktreeName: "foo", WorktreeBranch: "foo"}, cfg)
	if strings.Count(got.PlainText, "foo") != 1 {
		t.Errorf("expected branch not duplicated, got %q", got.PlainText)
	}
}

func TestContextWidget_Exceeds200kMarker(t *testing.T) {
	cfg := defaultCfg()
	ctx := &model.RenderContext{ContextPercent: 30, ContextWindowSize: 1000000, Exceeds200k: true}
	got := Context(ctx, cfg)
	if !strings.Contains(got.PlainText, ">200k") {
		t.Errorf("expected >200k marker, got %q", got.PlainText)
	}
}

func TestContextWidget_NoMarkerWhenUnder200k(t *testing.T) {
	cfg := defaultCfg()
	ctx := &model.RenderContext{ContextPercent: 30, ContextWindowSize: 1000000, Exceeds200k: false}
	got := Context(ctx, cfg)
	if strings.Contains(got.PlainText, ">200k") {
		t.Errorf("expected no marker, got %q", got.PlainText)
	}
}
