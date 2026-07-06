package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func speedCfg() *config.Config {
	cfg := config.LoadHud()
	cfg.Speed.WindowSecs = 30
	return cfg
}

// TestSpeedWidget_NoData returns empty when Transcript is nil.
func TestSpeedWidget_NoData_NilTranscript(t *testing.T) {
	ctx := &model.RenderContext{Transcript: nil}
	cfg := speedCfg()

	if got := Speed(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Speed with nil Transcript: expected empty, got %q", got.Text)
	}
}

// TestSpeedWidget_NoData_EmptySamples returns empty when there are no token samples.
func TestSpeedWidget_NoData_EmptySamples(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := speedCfg()

	if got := Speed(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Speed with empty TokenSamples: expected empty, got %q", got.Text)
	}
}

// TestSpeedWidget_SingleEntry averages a lone sample over the window: the
// fixed denominator needs no sample span, so a fresh burst renders immediately.
func TestSpeedWidget_SingleEntry(t *testing.T) {
	now := time.Now()
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: now, Tokens: 3000},
		},
	}}
	cfg := speedCfg()

	got := Speed(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("Speed with single windowed sample: expected non-empty output")
	}
	if !strings.Contains(got.Text, "100") {
		t.Errorf("Speed single sample: expected '100' (3000/30s window), got %q", got.Text)
	}
}

// TestSpeedWidget_MultipleEntries averages windowed tokens over the full
// window length, not the sample span, so clustered samples cannot spike the rate.
func TestSpeedWidget_MultipleEntries(t *testing.T) {
	base := time.Now()
	// 3000 tokens within a 60s window = 50 tok/s, regardless of sample spacing.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base.Add(-10 * time.Second), Tokens: 1500},
			{Timestamp: base, Tokens: 1500},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 60

	got := Speed(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("Speed with two samples: expected non-empty output")
	}
	if !strings.Contains(got.Text, "tok/s") {
		t.Errorf("Speed: expected 'tok/s' in output, got %q", got.Text)
	}
	if !strings.Contains(got.Text, "50") {
		t.Errorf("Speed: expected '50' (3000 tokens / 60s window) in output, got %q", got.Text)
	}
}

// TestSpeedWidget_WindowExpiry excludes samples outside the rolling window.
func TestSpeedWidget_WindowExpiry(t *testing.T) {
	base := time.Now()
	// Old sample (60s ago) with 10000 tokens — excluded by the 30s window.
	// Remaining 990 tokens / 30s window = 33 tok/s.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base.Add(-60 * time.Second), Tokens: 10000},
			{Timestamp: base.Add(-5 * time.Second), Tokens: 495},
			{Timestamp: base, Tokens: 495},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 30

	got := Speed(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("Speed with windowed samples: expected non-empty output")
	}
	if !strings.Contains(got.Text, "33") {
		t.Errorf("Speed window expiry: expected '33 tok/s' (990/30s), got %q", got.Text)
	}
	// The large old sample (10000 tokens) must not inflate the rate.
	if strings.Contains(got.Text, "10") && strings.Contains(got.Text, "k") {
		t.Errorf("Speed window expiry: old sample inflated rate, got %q", got.Text)
	}
}

// TestSpeedWidget_TrendArrows shows ↑ when the recent half-window leads the
// older half, ↓ when it trails, and no arrow when volume is steady.
func TestSpeedWidget_TrendArrows(t *testing.T) {
	cases := []struct {
		name          string
		olderTokens   int // sample in the older half (-20s of a 30s window)
		recentTokens  int // sample in the recent half (-5s)
		wantArrow     string
		unwantedArrow string
	}{
		{"ramping up", 500, 2000, "↑", "↓"},
		{"slowing down", 2000, 500, "↓", "↑"},
		{"steady", 1000, 1000, "", "↑↓"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := time.Now()
			ctx := &model.RenderContext{Transcript: &model.TranscriptData{
				TokenSamples: []model.TokenSample{
					{Timestamp: base.Add(-20 * time.Second), Tokens: tc.olderTokens},
					{Timestamp: base.Add(-5 * time.Second), Tokens: tc.recentTokens},
				},
			}}
			cfg := speedCfg()
			cfg.Speed.WindowSecs = 30

			got := Speed(ctx, cfg)
			if got.IsEmpty() {
				t.Fatal("Speed: expected non-empty output")
			}
			if tc.wantArrow != "" && !strings.Contains(got.PlainText, tc.wantArrow) {
				t.Errorf("expected %q in output, got %q", tc.wantArrow, got.PlainText)
			}
			for _, r := range tc.unwantedArrow {
				if strings.ContainsRune(got.PlainText, r) {
					t.Errorf("unexpected %q in output %q", string(r), got.PlainText)
				}
			}
		})
	}
}

// TestSpeedWidget_ZeroWindowUsesSessionAverage uses the full history when window_secs=0.
func TestSpeedWidget_ZeroWindowUsesSessionAverage(t *testing.T) {
	base := time.Now()
	// 3000 tokens over 20 seconds = 150 tok/s.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base, Tokens: 1000},
			{Timestamp: base.Add(10 * time.Second), Tokens: 1000},
			{Timestamp: base.Add(20 * time.Second), Tokens: 1000},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 0 // session average

	got := Speed(ctx, cfg)
	if got.IsEmpty() {
		t.Fatal("Speed session average: expected non-empty output")
	}
	if !strings.Contains(got.Text, "tok/s") {
		t.Errorf("Speed: expected 'tok/s' in output, got %q", got.Text)
	}
	// 3000 / 20 = 150 tok/s
	if !strings.Contains(got.Text, "150") {
		t.Errorf("Speed session average: expected '150' tok/s, got %q", got.Text)
	}
}

// TestSpeedWidget_StaleSamplesHidden returns empty when all samples are older
// than the window. Regression: the window must anchor at wall-clock now, not at
// the newest sample, or the last burst's rate stays on screen forever under
// timer-driven statusline refreshes.
func TestSpeedWidget_StaleSamplesHidden(t *testing.T) {
	base := time.Now()
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base.Add(-90 * time.Second), Tokens: 1000},
			{Timestamp: base.Add(-60 * time.Second), Tokens: 1000},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 30

	if got := Speed(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Speed with only stale samples: expected empty, got %q", got.Text)
	}
}

// TestSpeedWidget_RegisteredInRegistry verifies the 'speed' widget is in the registry.
func TestSpeedWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["speed"]; !ok {
		t.Error("Registry missing 'speed' widget")
	}
}
