package widget

import (
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestExec_EmptyOutput(t *testing.T) {
	ctx := &model.RenderContext{}
	got := Exec(ctx, &config.Config{})
	if !got.IsEmpty() {
		t.Errorf("Exec with no output = %+v, want empty", got)
	}
}

func TestExec_PassesTextVerbatim(t *testing.T) {
	styled := "\x1b[32m▂ 31%\x1b[0m"
	ctx := &model.RenderContext{ExecOutput: styled}
	got := Exec(ctx, &config.Config{})
	if got.Text != styled {
		t.Errorf("Exec Text = %q, want %q", got.Text, styled)
	}
	if got.PlainText != "▂ 31%" {
		t.Errorf("Exec PlainText = %q, want ANSI-stripped %q", got.PlainText, "▂ 31%")
	}
}
