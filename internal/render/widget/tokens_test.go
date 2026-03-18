package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestTokensWidget_HighCacheHit(t *testing.T) {
	ctx := &model.RenderContext{
		InputTokens:   1,
		CacheCreation: 557,
		CacheRead:     116190,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty output")
	}

	// total = 116748 → "116k tok", cache ratio = 116190/(116190+557) = 99%
	if !strings.Contains(got.PlainText, "116k tok") {
		t.Errorf("expected '116k tok', got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "99% cached") {
		t.Errorf("expected '99%% cached', got %q", got.PlainText)
	}
}

func TestTokensWidget_CacheBust(t *testing.T) {
	// Cache bust: high creation, low read — cache was rebuilt.
	ctx := &model.RenderContext{
		InputTokens:   500,
		CacheCreation: 80000,
		CacheRead:     20000,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)

	// cache ratio = 20000/(80000+20000) = 20%
	if !strings.Contains(got.PlainText, "20% cached") {
		t.Errorf("expected '20%% cached' during cache bust, got %q", got.PlainText)
	}
}

func TestTokensWidget_NoCacheableTokens(t *testing.T) {
	// Only uncacheable tokens, no cache activity — no ratio shown.
	ctx := &model.RenderContext{
		InputTokens:   5000,
		CacheCreation: 0,
		CacheRead:     0,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty output")
	}

	if got.PlainText != "5.0k tok" {
		t.Errorf("expected '5.0k tok' with no cache info, got %q", got.PlainText)
	}
	if strings.Contains(got.PlainText, "cached") {
		t.Errorf("should not show cache ratio when no cacheable tokens, got %q", got.PlainText)
	}
}

func TestTokensWidget_AllZero(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Tokens(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty for zero tokens, got %q", got.Text)
	}
}

func TestTokensWidget_FullCacheHit(t *testing.T) {
	// 100% cache read, no creation.
	ctx := &model.RenderContext{
		InputTokens:   0,
		CacheCreation: 0,
		CacheRead:     100000,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)
	if !strings.Contains(got.PlainText, "100% cached") {
		t.Errorf("expected '100%% cached', got %q", got.PlainText)
	}
}

func TestTokensWidget_AllCreationNoRead(t *testing.T) {
	// First call in session: everything is cache creation, nothing read yet.
	ctx := &model.RenderContext{
		InputTokens:   200,
		CacheCreation: 50000,
		CacheRead:     0,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)
	if !strings.Contains(got.PlainText, "0% cached") {
		t.Errorf("expected '0%% cached' on first call, got %q", got.PlainText)
	}
}

func TestTokensWidget_RegisteredInRegistry(t *testing.T) {
	fn, ok := Registry["tokens"]
	if !ok {
		t.Fatal("'tokens' not found in widget.Registry")
	}
	if fn == nil {
		t.Fatal("'tokens' registry entry is nil")
	}
}

func TestTokensWidget_DualOutput(t *testing.T) {
	ctx := &model.RenderContext{
		InputTokens:   3,
		CacheCreation: 370,
		CacheRead:     125464,
	}
	cfg := defaultCfg()

	got := Tokens(ctx, cfg)
	if got.FgColor != "8" {
		t.Errorf("FgColor: expected '8', got %q", got.FgColor)
	}
	if got.PlainText == "" {
		t.Error("PlainText should be set")
	}
	// Text should be the MutedStyle-rendered version of PlainText.
	if !strings.Contains(got.Text, "125k tok") {
		t.Errorf("Text should contain total, got %q", got.Text)
	}
}
