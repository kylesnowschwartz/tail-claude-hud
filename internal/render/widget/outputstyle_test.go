package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestOutputStyleWidget_PresentStyleName(t *testing.T) {
	ctx := &model.RenderContext{OutputStyle: "default"}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := OutputStyle(ctx, cfg)
	icons := IconsFor("ascii")

	// Text must be dim-styled and contain the edit icon + style name.
	want := dimStyle.Render(icons.Edit + " " + "default")
	if got.Text != want {
		t.Errorf("OutputStyle: expected %q, got %q", want, got.Text)
	}
}

func TestOutputStyleWidget_EmptyString(t *testing.T) {
	ctx := &model.RenderContext{OutputStyle: ""}
	cfg := defaultCfg()

	if got := OutputStyle(ctx, cfg); !got.IsEmpty() {
		t.Errorf("OutputStyle with empty string: expected empty, got %q", got.Text)
	}
}

func TestOutputStyleWidget_NilContext(t *testing.T) {
	// Simulate nil-equivalent: RenderContext with zero-value OutputStyle.
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := OutputStyle(ctx, cfg); !got.IsEmpty() {
		t.Errorf("OutputStyle with zero-value context: expected empty, got %q", got.Text)
	}
}

func TestOutputStyleWidget_RendersEditIconAndStyleName(t *testing.T) {
	ctx := &model.RenderContext{OutputStyle: "concise"}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := OutputStyle(ctx, cfg)
	icons := IconsFor("ascii")

	// The rendered text must contain the edit icon and the style name.
	if !strings.Contains(got.Text, icons.Edit) {
		t.Errorf("OutputStyle: expected edit icon %q in output, got %q", icons.Edit, got.Text)
	}
	if !strings.Contains(got.Text, "concise") {
		t.Errorf("OutputStyle: expected style name 'concise' in output, got %q", got.Text)
	}
}

func TestOutputStyleWidget_VariousStyleNames(t *testing.T) {
	tests := []struct {
		name  string
		style string
	}{
		{"default style", "default"},
		{"concise style", "concise"},
		{"verbose style", "verbose"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &model.RenderContext{OutputStyle: tt.style}
			cfg := defaultCfg()
			cfg.Style.Icons = "ascii"
			got := OutputStyle(ctx, cfg)
			icons := IconsFor("ascii")
			want := dimStyle.Render(icons.Edit + " " + tt.style)
			if got.Text != want {
				t.Errorf("OutputStyle(%q): expected %q, got %q", tt.style, want, got.Text)
			}
		})
	}
}

func TestOutputStyleWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["outputstyle"]; !ok {
		t.Error("Registry missing 'outputstyle' widget")
	}
}
