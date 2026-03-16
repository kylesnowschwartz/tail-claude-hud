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

	if got := Speed(ctx, cfg); got != "" {
		t.Errorf("Speed with nil Transcript: expected empty, got %q", got)
	}
}

// TestSpeedWidget_NoData_EmptySamples returns empty when there are no token samples.
func TestSpeedWidget_NoData_EmptySamples(t *testing.T) {
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{}}
	cfg := speedCfg()

	if got := Speed(ctx, cfg); got != "" {
		t.Errorf("Speed with empty TokenSamples: expected empty, got %q", got)
	}
}

// TestSpeedWidget_SingleEntry returns empty because a single sample has no elapsed time.
func TestSpeedWidget_SingleEntry(t *testing.T) {
	now := time.Now()
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: now, Tokens: 5000},
		},
	}}
	cfg := speedCfg()

	// Single sample: no elapsed time, so speed cannot be computed.
	if got := Speed(ctx, cfg); got != "" {
		t.Errorf("Speed with single sample: expected empty (no time span), got %q", got)
	}
}

// TestSpeedWidget_MultipleEntries calculates tok/s from multiple samples.
func TestSpeedWidget_MultipleEntries(t *testing.T) {
	base := time.Now()
	// Two samples 10 seconds apart with 1000 tokens each = 200 tok/s (2000 / 10).
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base, Tokens: 1000},
			{Timestamp: base.Add(10 * time.Second), Tokens: 1000},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 60 // wide window so both samples are included

	got := Speed(ctx, cfg)
	if got == "" {
		t.Fatal("Speed with two samples: expected non-empty output")
	}
	if !strings.Contains(got, "tok/s") {
		t.Errorf("Speed: expected 'tok/s' in output, got %q", got)
	}
	// 2000 tokens over 10 seconds = 200 tok/s → formatted as "200 tok/s"
	if !strings.Contains(got, "200") {
		t.Errorf("Speed: expected '200' in output, got %q", got)
	}
}

// TestSpeedWidget_WindowExpiry excludes samples outside the rolling window.
func TestSpeedWidget_WindowExpiry(t *testing.T) {
	base := time.Now()
	// Old sample (60s ago) with 10000 tokens — should be excluded by 30s window.
	// Recent sample (5s ago) with 500 tokens and one at 0s with 500 tokens = 1000/5s = 200 tok/s.
	ctx := &model.RenderContext{Transcript: &model.TranscriptData{
		TokenSamples: []model.TokenSample{
			{Timestamp: base.Add(-60 * time.Second), Tokens: 10000},
			{Timestamp: base.Add(-5 * time.Second), Tokens: 500},
			{Timestamp: base, Tokens: 500},
		},
	}}
	cfg := speedCfg()
	cfg.Speed.WindowSecs = 30

	got := Speed(ctx, cfg)
	if got == "" {
		t.Fatal("Speed with windowed samples: expected non-empty output")
	}
	if !strings.Contains(got, "tok/s") {
		t.Errorf("Speed: expected 'tok/s' in output, got %q", got)
	}
	// The old sample should be excluded. 1000 tokens over 5s = 200 tok/s.
	if !strings.Contains(got, "200") {
		t.Errorf("Speed window expiry: expected '200 tok/s', got %q", got)
	}
	// The large old sample (10000 tokens) must not inflate the rate.
	if strings.Contains(got, "10") && strings.Contains(got, "k") {
		t.Errorf("Speed window expiry: old sample inflated rate, got %q", got)
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
	if got == "" {
		t.Fatal("Speed session average: expected non-empty output")
	}
	if !strings.Contains(got, "tok/s") {
		t.Errorf("Speed: expected 'tok/s' in output, got %q", got)
	}
	// 3000 / 20 = 150 tok/s
	if !strings.Contains(got, "150") {
		t.Errorf("Speed session average: expected '150' tok/s, got %q", got)
	}
}

// TestSpeedWidget_RegisteredInRegistry verifies the 'speed' widget is in the registry.
func TestSpeedWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["speed"]; !ok {
		t.Error("Registry missing 'speed' widget")
	}
}
