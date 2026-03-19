package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestThinkingWidget_NilTranscriptReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: nil}
	cfg := defaultCfg()

	if got := Thinking(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Thinking with nil Transcript: expected empty, got %q", got.Text)
	}
}

func TestThinkingWidget_ActiveShowsYellowIcon(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		ThinkingActive: true,
		ThinkingCount:  3,
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Thinking(ctx, cfg).Text
	icons := IconsFor("ascii")

	if !strings.Contains(got, icons.Thinking) {
		t.Errorf("Thinking active: expected thinking icon %q, got %q", icons.Thinking, got)
	}

	// Must match exactly the yellow-styled icon — no count appended.
	want := yellowStyle.Render(icons.Thinking)
	if got != want {
		t.Errorf("Thinking active: expected %q, got %q", want, got)
	}
}

func TestThinkingWidget_InactiveZeroCountReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		ThinkingActive: false,
		ThinkingCount:  0,
	}}
	cfg := defaultCfg()

	if got := Thinking(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Thinking inactive with zero count: expected empty, got %q", got.Text)
	}
}

func TestThinkingWidget_InactiveWithCountShowsDim(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		ThinkingActive: false,
		ThinkingCount:  3,
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Thinking(ctx, cfg).Text
	icons := IconsFor("ascii")

	if !strings.Contains(got, icons.Thinking) {
		t.Errorf("Thinking dim count: expected thinking icon %q in output, got %q", icons.Thinking, got)
	}
	if !strings.Contains(got, "3") {
		t.Errorf("Thinking dim count: expected count '3' in output, got %q", got)
	}

	// Must match exactly the dim-styled icon+count.
	want := DimStyle.Render(icons.Thinking + "3")
	if got != want {
		t.Errorf("Thinking dim count: expected %q, got %q", want, got)
	}
}

func TestThinkingWidget_ActiveTakesPriorityOverCount(t *testing.T) {
	// When active, we show the yellow icon even if ThinkingCount > 0.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		ThinkingActive: true,
		ThinkingCount:  5,
	}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Thinking(ctx, cfg).Text
	icons := IconsFor("ascii")

	want := yellowStyle.Render(icons.Thinking)
	if got != want {
		t.Errorf("Thinking active+count: expected yellow icon only %q, got %q", want, got)
	}
}

func TestThinkingWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["thinking"]; !ok {
		t.Error("Registry missing 'thinking' widget")
	}
}
