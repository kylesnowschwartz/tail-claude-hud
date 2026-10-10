package widget

import (
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var cacheSentAt = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)

func cacheCtx(ttl time.Duration) *model.RenderContext {
	return &model.RenderContext{Transcript: &model.TranscriptData{
		LastRequestAt: cacheSentAt,
		CacheTTL:      ttl,
	}}
}

func TestCacheTTL_EmptyBeforeAnyRequest(t *testing.T) {
	cfg := config.LoadHud()
	for name, ctx := range map[string]*model.RenderContext{
		"nil transcript": {},
		"no request":     {Transcript: &model.TranscriptData{}},
	} {
		if got := cacheTTLAt(ctx, cfg, cacheSentAt); !got.IsEmpty() {
			t.Errorf("%s: expected empty, got %q", name, got.PlainText)
		}
	}
}

func TestCacheTTL_DisplayStates(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Style.Colors.Warning = "yellow"

	tests := []struct {
		name    string
		ttl     time.Duration
		elapsed time.Duration
		want    string
		fg      string
	}{
		{"fresh 1h", time.Hour, 0, "● 60m", "8"},
		{"1h minutes round up", time.Hour, 13*time.Minute + 30*time.Second, "◕ 47m", "8"},
		{"1h three-quarters left", time.Hour, 15 * time.Minute, "◕ 45m", "8"},
		{"1h half left", time.Hour, 30 * time.Minute, "◑ 30m", "8"},
		{"1h quarter left", time.Hour, 45 * time.Minute, "◔ 15m", "8"},
		{"1h warn at 20%", time.Hour, 48 * time.Minute, "◔ 12m", "3"},
		{"1h nearly empty", time.Hour, 55 * time.Minute, "○ 5m", "3"},
		{"last minute shows seconds", time.Hour, 59*time.Minute + 18*time.Second, "○ 0:42", "3"},
		{"sub-second rounds up to a second", time.Hour, time.Hour - 100*time.Millisecond, "○ 0:01", "3"},
		{"just under a minute reads 1m", time.Hour, 59*time.Minute + 200*time.Millisecond, "○ 1m", "3"},
		{"expired is cold", time.Hour, time.Hour, "◌ Cold", "8"},
		{"long expired is cold", time.Hour, 3 * time.Hour, "◌ Cold", "8"},
		{"5m write", 5 * time.Minute, 2 * time.Minute, "◑ 3m", "8"},
		{"5m warn", 5 * time.Minute, 4 * time.Minute, "◔ 1m", "3"},
		{"no write seen defaults to 1h", 0, 10 * time.Minute, "◕ 50m", "8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheTTLAt(cacheCtx(tt.ttl), cfg, cacheSentAt.Add(tt.elapsed))
			if got.PlainText != tt.want {
				t.Errorf("PlainText = %q, want %q", got.PlainText, tt.want)
			}
			if got.FgColor != tt.fg {
				t.Errorf("FgColor = %q, want %q", got.FgColor, tt.fg)
			}
			if got.Text == "" {
				t.Error("Text is empty")
			}
		})
	}
}

func TestCacheTTL_SettingOverridesTranscript(t *testing.T) {
	cfg := config.LoadHud()
	now := cacheSentAt.Add(2 * time.Minute)

	cfg.CacheTTL.TTL = "5m"
	if got := cacheTTLAt(cacheCtx(time.Hour), cfg, now).PlainText; got != "◑ 3m" {
		t.Errorf(`ttl="5m": got %q, want "◑ 3m"`, got)
	}

	cfg.CacheTTL.TTL = "1h"
	if got := cacheTTLAt(cacheCtx(5*time.Minute), cfg, now).PlainText; got != "● 58m" {
		t.Errorf(`ttl="1h": got %q, want "● 58m"`, got)
	}

	cfg.CacheTTL.TTL = "bogus"
	if got := cacheTTLAt(cacheCtx(5*time.Minute), cfg, now).PlainText; got != "◑ 3m" {
		t.Errorf(`ttl="bogus" follows transcript: got %q, want "◑ 3m"`, got)
	}
}

func TestCacheTTL_WarnUsesConfiguredWarningColor(t *testing.T) {
	cfg := config.LoadHud()
	cfg.Style.Colors.Warning = "214"
	got := cacheTTLAt(cacheCtx(time.Hour), cfg, cacheSentAt.Add(50*time.Minute))
	if got.FgColor != "214" {
		t.Errorf("FgColor = %q, want configured warning %q", got.FgColor, "214")
	}
}
