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

// TestGitWidget_ThemeKeepsSemanticColors pins the composite-widget contract:
// the theme's fg colors the branch identity text, while decorators keep
// their own styling instead of being flattened. Regression for theme fg
// overrides erasing per-element colors (FgColor must stay empty so the
// renderer passes the pre-styled text through).
func TestGitWidget_ThemeKeepsSemanticColors(t *testing.T) {
	cfg := config.LoadHud()
	cfg.ResolvedTheme = theme.Theme{"git": {Fg: "75"}}

	got := Git(gitCtx(&model.GitStatus{
		Branch: "main",
		Dirty:  true,
	}), cfg)

	if got.FgColor != "" {
		t.Errorf("FgColor = %q, want empty (composite widget must opt out of renderer-level theme flattening)", got.FgColor)
	}
	if want := DimStyle.Render("*"); !strings.Contains(got.Text, want) {
		t.Errorf("expected dim-styled dirty indicator in Text, got %q", got.Text)
	}
	branchStr := IconsFor(cfg.Style.Icons).Branch + " main"
	if want := themeFgStyle(cfg, "git", gitBranchStyle).Render(branchStr); !strings.Contains(got.Text, want) {
		t.Errorf("expected theme-fg-styled branch in Text, got %q", got.Text)
	}
}
