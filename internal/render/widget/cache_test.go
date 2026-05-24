package widget

import (
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestCache_ReturnsEmptyWhenNoCacheableTokens(t *testing.T) {
	ctx := &model.RenderContext{
		CacheRead:    0,
		CacheCreation: 0,
		CacheSamples:  nil,
	}
	cfg := (*config.Config)(nil) // not used by Cache widget

	result := Cache(ctx, cfg)
	if !result.IsEmpty() {
		t.Errorf("expected empty result when no cacheable tokens, got: %+v", result)
	}
}

func TestCache_ShowsCurrentRate(t *testing.T) {
	ctx := &model.RenderContext{
		CacheRead:    80,
		CacheCreation: 20,
		CacheSamples:  nil,
	}
	cfg := (*config.Config)(nil)

	result := Cache(ctx, cfg)
	if result.IsEmpty() {
		t.Fatal("expected non-empty result")
	}
	// Current rate = 80 / (80+20) = 80%
	expected := "cache:80%"
	if result.PlainText != expected {
		t.Errorf("expected %q, got %q", expected, result.PlainText)
	}
}

func TestCache_ColorCoding(t *testing.T) {
	tests := []struct {
		name     string
		read     int
		creation int
		wantColor string
	}{
		{"red when < 40", 30, 70, "9"},
		{"yellow when 40-60", 50, 50, "11"},
		{"white when > 60", 70, 30, ""},
		{"yellow when exactly 60", 60, 40, "11"},
		{"red when exactly 39", 39, 61, "9"},
		{"yellow when exactly 40", 40, 60, "11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &model.RenderContext{
				CacheRead:    tt.read,
				CacheCreation: tt.creation,
			}
			result := Cache(ctx, nil)
			if result.FgColor != tt.wantColor {
				t.Errorf("Cache(%d,%d): FgColor = %q, want %q", tt.read, tt.creation, result.FgColor, tt.wantColor)
			}
		})
	}
}

func TestCache_RollingAverages(t *testing.T) {
	now := time.Now()
	samples := []model.CacheSample{
		{Timestamp: now.Add(-10 * time.Minute), CacheRate: 90},
		{Timestamp: now.Add(-8 * time.Minute), CacheRate: 80},
		{Timestamp: now.Add(-6 * time.Minute), CacheRate: 70},
		{Timestamp: now.Add(-4 * time.Minute), CacheRate: 60},
		{Timestamp: now.Add(-2 * time.Minute), CacheRate: 50},
		{Timestamp: now, CacheRate: 40},
	}

	ctx := &model.RenderContext{
		CacheRead:    40,
		CacheCreation: 60,
		CacheSamples:  samples,
	}

	result := Cache(ctx, nil)
	// 5min window: last 3 samples (60, 50, 40) → avg = 50
	// 1h window: all samples → avg = (90+80+70+60+50+40)/6 = 65
	if result.PlainText == "" {
		t.Fatal("expected non-empty result")
	}
	// Check that the plain text contains the expected parts
	got := result.PlainText
	expectedParts := []string{"cache:40%", "5min:50%", "1h:65%"}
	for _, part := range expectedParts {
		if !contains(got, part) {
			t.Errorf("PlainText = %q, want it to contain %q", got, part)
		}
	}
}

func TestCache_NoRollingWhenNoSamples(t *testing.T) {
	ctx := &model.RenderContext{
		CacheRead:    80,
		CacheCreation: 20,
		CacheSamples:  nil, // no samples → no rolling averages
	}

	result := Cache(ctx, nil)
	// Should only show current rate
	want := "cache:80%"
	if result.PlainText != want {
		t.Errorf("expected %q, got %q", want, result.PlainText)
	}
}

func TestCache_SamplesOutsideWindow(t *testing.T) {
	now := time.Now()
	samples := []model.CacheSample{
		// All samples are older than both 5min and 1h windows
		{Timestamp: now.Add(-2 * time.Hour), CacheRate: 90},
		{Timestamp: now.Add(-90 * time.Minute), CacheRate: 80},
	}

	ctx := &model.RenderContext{
		CacheRead:    40,
		CacheCreation: 60,
		CacheSamples:  samples,
	}

	result := Cache(ctx, nil)
	// No samples within 5min or 1h → no rolling averages
	want := "cache:40%"
	if result.PlainText != want {
		t.Errorf("expected %q (no rolling averages), got %q", want, result.PlainText)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && search(s, substr))
}

func search(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
