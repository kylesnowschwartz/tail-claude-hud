package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestUsage_NilUsageReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Usage: nil}
	cfg := defaultCfg()

	if got := Usage(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty for nil usage, got %q", got.Text)
	}
}

func TestUsage_LimitReached(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			FiveHourPercent: 100,
			FiveHourResetAt: time.Now().Add(45 * time.Minute),
		},
	}
	cfg := defaultCfg()

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for limit reached")
	}
	if !strings.Contains(got.PlainText, "Limit reached") {
		t.Errorf("expected 'Limit reached' in output, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "resets") {
		t.Errorf("expected reset countdown in output, got %q", got.PlainText)
	}
}

func TestUsage_BarModeShowsLabeledBars(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			FiveHourPercent: 62,
			SevenDayPercent: 85,
		},
	}
	cfg := defaultCfg()
	cfg.Usage.Display = "bar"
	cfg.Usage.BarWidth = 8

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty in bar mode")
	}
	// Labels distinguish the windows since a bar alone cannot.
	if !strings.Contains(got.PlainText, "5h") || !strings.Contains(got.PlainText, "7d") {
		t.Errorf("expected 5h and 7d labels, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "█") {
		t.Errorf("expected bar characters, got %q", got.PlainText)
	}
	// 62% of 8 cells = 4 filled; 85% = 6 filled.
	if !strings.Contains(got.PlainText, "████░░░░") {
		t.Errorf("expected 4/8 bar for 62%%, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "██████░░") {
		t.Errorf("expected 6/8 bar for 85%%, got %q", got.PlainText)
	}
	// Reset countdown is a text-mode element.
	if strings.Contains(got.PlainText, "(") {
		t.Errorf("expected no reset countdown in bar mode, got %q", got.PlainText)
	}
}

func TestUsage_BelowThresholdHides(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			FiveHourPercent: 10,
			SevenDayPercent: 5,
		},
	}
	cfg := defaultCfg()
	cfg.Usage.FiveHourThreshold = 50 // threshold higher than usage

	if got := Usage(ctx, cfg); !got.IsEmpty() {
		t.Errorf("expected empty below threshold, got %q", got.PlainText)
	}
}

func TestUsage_SevenDayShownAboveThreshold(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			FiveHourPercent: 30,
			SevenDayPercent: 85,
			SevenDayResetAt: time.Now().Add(48 * time.Hour),
		},
	}
	cfg := defaultCfg()
	cfg.Usage.FiveHourThreshold = 0
	cfg.Usage.SevenDayThreshold = 80

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty")
	}
	// Two windows should produce a separator in the output.
	if !strings.Contains(got.PlainText, "|") {
		t.Errorf("expected separator for two windows, got %q", got.PlainText)
	}
	// Second window should have a reset countdown.
	if !strings.Contains(got.PlainText, "2d") {
		t.Errorf("expected '2d' reset in output, got %q", got.PlainText)
	}
}

func TestUsage_SevenDayHiddenBelowThreshold(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			FiveHourPercent: 30,
			SevenDayPercent: 50,
		},
	}
	cfg := defaultCfg()
	cfg.Usage.FiveHourThreshold = 0
	cfg.Usage.SevenDayThreshold = 80

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty (5h is above threshold)")
	}
	if strings.Contains(got.PlainText, "7d") {
		t.Errorf("expected no '7d' when below seven_day_threshold, got %q", got.PlainText)
	}
}

func TestUsage_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["usage"]; !ok {
		t.Error("Registry missing 'usage' widget")
	}
}

func TestFormatResetTime(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"past", -1 * time.Hour, ""},
		{"zero", 0, ""},
		{"30m", 30 * time.Minute, "30m"},
		{"1h30m", 90 * time.Minute, "1h 30m"},
		{"2h", 2 * time.Hour, "2h"},
		{"1d5h", 29 * time.Hour, "1d 5h"},
		{"2d", 48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resetAt time.Time
			if tt.offset > 0 {
				resetAt = time.Now().Add(tt.offset)
			}
			got := formatResetTime(resetAt)
			if got != tt.want {
				t.Errorf("formatResetTime(now+%v) = %q, want %q", tt.offset, got, tt.want)
			}
		})
	}
}
