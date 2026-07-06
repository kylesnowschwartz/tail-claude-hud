package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/theme"
)

func gitCtx(g *model.GitStatus) *model.RenderContext {
	return &model.RenderContext{Git: g}
}

// TestGitWidget_NilStatus returns empty when no git data was gathered.
func TestGitWidget_NilStatus(t *testing.T) {
	if got := Git(gitCtx(nil), config.LoadHud()); !got.IsEmpty() {
		t.Errorf("Git with nil status: expected empty, got %q", got.Text)
	}
}

// TestGitWidget_LineStats renders +added/-removed when line_stats is enabled
// and the tree is dirty.
func TestGitWidget_LineStats(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Git.LineStats = true

	got := Git(gitCtx(&model.GitStatus{
		Branch:       "main",
		Dirty:        true,
		Modified:     2,
		LinesAdded:   100,
		LinesRemoved: 55,
	}), cfg)

	if !strings.Contains(got.PlainText, "+100") {
		t.Errorf("expected '+100' in output, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "-55") {
		t.Errorf("expected '-55' in output, got %q", got.PlainText)
	}
}

// TestGitWidget_LineStats_Disabled omits line deltas when line_stats is off
// (the default), even if the gather stage populated them.
func TestGitWidget_LineStats_Disabled(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Git.LineStats = false // explicit: LoadHud overlays the user's real config

	got := Git(gitCtx(&model.GitStatus{
		Branch:       "main",
		Dirty:        true,
		LinesAdded:   100,
		LinesRemoved: 55,
	}), cfg)

	if strings.Contains(got.PlainText, "+100") || strings.Contains(got.PlainText, "-55") {
		t.Errorf("line stats rendered with line_stats=false: %q", got.PlainText)
	}
}

// TestGitWidget_ThemeKeepsSemanticColors pins the composite-widget contract:
// the theme's fg colors the branch identity text, while the green/red line
// deltas keep their own styling instead of being flattened. Regression for
// theme fg overrides erasing the +N/-M colors (FgColor must stay empty so
// the renderer passes the pre-styled text through).
func TestGitWidget_ThemeKeepsSemanticColors(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Git.LineStats = true
	cfg.ResolvedTheme = theme.Theme{"git": {Fg: "75"}}

	got := Git(gitCtx(&model.GitStatus{
		Branch:       "main",
		Dirty:        true,
		LinesAdded:   100,
		LinesRemoved: 55,
	}), cfg)

	if got.FgColor != "" {
		t.Errorf("FgColor = %q, want empty (composite widget must opt out of renderer-level theme flattening)", got.FgColor)
	}
	if want := linesAddedStyle.Render("+100"); !strings.Contains(got.Text, want) {
		t.Errorf("expected green-styled %q in Text, got %q", "+100", got.Text)
	}
	if want := linesRemovedStyle.Render("-55"); !strings.Contains(got.Text, want) {
		t.Errorf("expected red-styled %q in Text, got %q", "-55", got.Text)
	}
	branchStr := IconsFor(cfg.Style.Icons).Branch + " main"
	if want := themeFgStyle(cfg, "git", gitBranchStyle).Render(branchStr); !strings.Contains(got.Text, want) {
		t.Errorf("expected theme-fg-styled branch in Text, got %q", got.Text)
	}
}

// TestGitWidget_LineStats_ZeroCountsHidden shows no "+0"/"-0" noise when a
// dirty tree has deltas only on one side (or none, e.g. untracked-only).
func TestGitWidget_LineStats_ZeroCountsHidden(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Git.LineStats = true

	got := Git(gitCtx(&model.GitStatus{
		Branch:     "main",
		Dirty:      true,
		LinesAdded: 12,
	}), cfg)

	if !strings.Contains(got.PlainText, "+12") {
		t.Errorf("expected '+12' in output, got %q", got.PlainText)
	}
	if strings.Contains(got.PlainText, "-0") || strings.Contains(got.PlainText, "+0") {
		t.Errorf("zero-count delta rendered: %q", got.PlainText)
	}
}
