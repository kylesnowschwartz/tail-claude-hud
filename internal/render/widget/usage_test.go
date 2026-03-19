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

func TestUsage_APIUnavailableShowsWarning(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			PlanName:       "Pro",
			APIUnavailable: true,
			APIError:       "timeout",
		},
	}
	cfg := defaultCfg()

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for API unavailable")
	}
	if !strings.Contains(got.PlainText, "Usage") {
		t.Errorf("expected 'Usage' in output, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "timeout") {
		t.Errorf("expected 'timeout' hint in output, got %q", got.PlainText)
	}
}

func TestUsage_RateLimitedShowsSyncing(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			PlanName:        "Pro",
			FiveHourPercent: 25,
			SevenDayPercent: 40,
			APIError:        "rate-limited",
		},
	}
	cfg := defaultCfg()
	cfg.Usage.FiveHourThreshold = 0

	got := Usage(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("expected non-empty for rate-limited with data")
	}
	if !strings.Contains(got.PlainText, "syncing") {
		t.Errorf("expected 'syncing' in output, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "25%") {
		t.Errorf("expected '25%%' in output, got %q", got.PlainText)
	}
}

func TestUsage_LimitReached(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			PlanName:        "Pro",
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

func TestUsage_BelowThresholdHides(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			PlanName:        "Pro",
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
			PlanName:        "Pro",
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
	if !strings.Contains(got.PlainText, "7d") {
		t.Errorf("expected '7d' in output, got %q", got.PlainText)
	}
	if !strings.Contains(got.PlainText, "85%") {
		t.Errorf("expected '85%%' in output, got %q", got.PlainText)
	}
}

func TestUsage_SevenDayHiddenBelowThreshold(t *testing.T) {
	ctx := &model.RenderContext{
		Usage: &model.UsageInfo{
			PlanName:        "Pro",
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

func TestFormatUsageError(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"rate-limited", "(syncing...)"},
		{"http-401", "(401)"},
		{"timeout", "(timeout)"},
	}
	for _, tt := range tests {
		got := formatUsageError(tt.input)
		if got != tt.want {
			t.Errorf("formatUsageError(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
