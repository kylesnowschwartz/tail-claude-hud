package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestSessions_EmptyReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Sessions: []model.SessionInfo{}}
	cfg := defaultCfg()

	if got := Sessions(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty for no sessions, got %q", got.Text)
	}
}

func TestSessions_NilReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Sessions: nil}
	cfg := defaultCfg()

	if got := Sessions(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty for nil sessions, got %q", got.Text)
	}
}

func TestSessions_OneRunning(t *testing.T) {
	ctx := &model.RenderContext{
		Sessions: []model.SessionInfo{
			{SessionID: "s1", Project: "my-project", Running: true},
		},
	}
	cfg := defaultCfg()

	got := Sessions(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for one running session")
	}
	if !strings.Contains(got.PlainText, "\u25CF") {
		t.Errorf("expected filled dot, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "my-project") {
		t.Errorf("expected project name, got %q", got.PlainText)
	}
}

func TestSessions_OneIdle(t *testing.T) {
	ctx := &model.RenderContext{
		Sessions: []model.SessionInfo{
			{SessionID: "s1", Project: "idle-proj", Running: false},
		},
	}
	cfg := defaultCfg()

	got := Sessions(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for one idle session")
	}
	if !strings.Contains(got.PlainText, "\u25CB") {
		t.Errorf("expected open dot, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "idle-proj") {
		t.Errorf("expected project name, got %q", got.PlainText)
	}
}

func TestSessions_Mixed(t *testing.T) {
	ctx := &model.RenderContext{
		Sessions: []model.SessionInfo{
			{SessionID: "s1", Project: "active", Running: true},
			{SessionID: "s2", Project: "paused", Running: false},
		},
	}
	cfg := defaultCfg()

	got := Sessions(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for mixed sessions")
	}
	if !strings.Contains(got.PlainText, "\u25CF") {
		t.Error("expected filled dot for running session")
	}
	if !strings.Contains(got.PlainText, "\u25CB") {
		t.Error("expected open dot for idle session")
	}
}

func TestSessions_ShowProjectFalse(t *testing.T) {
	ctx := &model.RenderContext{
		Sessions: []model.SessionInfo{
			{SessionID: "s1", Project: "my-project", Running: true},
		},
	}
	cfg := defaultCfg()
	cfg.Sessions.ShowProject = false

	got := Sessions(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty")
	}
	if strings.Contains(got.PlainText, "my-project") {
		t.Errorf("expected no project name when ShowProject=false, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "\u25CF") {
		t.Errorf("expected dot still present, got %q", got.PlainText)
	}
}
