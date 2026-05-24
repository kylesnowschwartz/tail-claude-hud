package cachestate

import (
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestCacheHitRate(t *testing.T) {
	tests := []struct {
		name       string
		cacheRead  int
		cacheCreation int
		want       int
	}{
		{"100% hit", 80, 0, 100},
		{"80% hit", 80, 20, 80},
		{"50% hit", 50, 50, 50},
		{"0% hit", 0, 100, 0},
		{"no cacheable tokens", 0, 0, 0},
		{"only creation", 0, 100, 0},
		{"only read", 100, 0, 100},
		{"exactly 40%", 40, 60, 40},
		{"exactly 60%", 60, 40, 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CacheHitRate(tt.cacheRead, tt.cacheCreation)
			if got != tt.want {
				t.Errorf("CacheHitRate(%d, %d) = %d, want %d", tt.cacheRead, tt.cacheCreation, got, tt.want)
			}
		})
	}
}

func TestCacheHitRate_OverflowSafe(t *testing.T) {
	// Ensure int64 arithmetic avoids overflow even with large token counts.
	// 1e9 * 100 = 1e11, well within int64 range.
	got := CacheHitRate(1_000_000_000, 0)
	if got != 100 {
		t.Errorf("CacheHitRate(1e9, 0) = %d, want 100", got)
	}
	got = CacheHitRate(1_000_000_000, 1_000_000_000)
	if got != 50 {
		t.Errorf("CacheHitRate(1e9, 1e9) = %d, want 50", got)
	}
}

func TestRollingAverage(t *testing.T) {
	now := time.Now()
	samples := []model.CacheSample{
		{Timestamp: now.Add(-10 * time.Minute), CacheRate: 90},
		{Timestamp: now.Add(-8 * time.Minute), CacheRate: 80},
		{Timestamp: now.Add(-6 * time.Minute), CacheRate: 70},
		{Timestamp: now.Add(-4 * time.Minute), CacheRate: 60},
		{Timestamp: now.Add(-2 * time.Minute), CacheRate: 50},
		{Timestamp: now, CacheRate: 40},
	}

	// 5min window: last 3 samples (60, 50, 40) → avg = 50
	got := RollingAverage(samples, 5*time.Minute)
	if got != 50 {
		t.Errorf("RollingAverage(5m) = %d, want 50", got)
	}

	// 1h window: all 6 samples → avg = (90+80+70+60+50+40)/6 = 65
	got = RollingAverage(samples, 1*time.Hour)
	if got != 65 {
		t.Errorf("RollingAverage(1h) = %d, want 65", got)
	}

	// Empty samples
	got = RollingAverage(nil, 5*time.Minute)
	if got != -1 {
		t.Errorf("RollingAverage(nil) = %d, want -1", got)
	}

	// All samples outside window
	oldSamples := []model.CacheSample{
		{Timestamp: now.Add(-2 * time.Hour), CacheRate: 90},
		{Timestamp: now.Add(-90 * time.Minute), CacheRate: 80},
	}
	got = RollingAverage(oldSamples, 1*time.Hour)
	if got != -1 {
		t.Errorf("RollingAverage(outside window) = %d, want -1", got)
	}
}

func TestState_AppendIfChanged(t *testing.T) {
	s := &State{Samples: []model.CacheSample{}}

	// First sample should be appended.
	s.AppendIfChanged(model.CacheSample{CacheRead: 80, CacheCreation: 20, InputTokens: 100})
	if len(s.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(s.Samples))
	}
	if s.Samples[0].CacheRate != 80 {
		t.Errorf("expected CacheRate 80, got %d", s.Samples[0].CacheRate)
	}

	// Duplicate values should NOT be appended (within threshold).
	s.AppendIfChanged(model.CacheSample{CacheRead: 80, CacheCreation: 20, InputTokens: 100})
	if len(s.Samples) != 1 {
		t.Errorf("duplicate sample should not be appended, got %d samples", len(s.Samples))
	}

	// Changed values should be appended.
	s.AppendIfChanged(model.CacheSample{CacheRead: 50, CacheCreation: 50, InputTokens: 200})
	if len(s.Samples) != 2 {
		t.Fatalf("changed sample should be appended, got %d samples", len(s.Samples))
	}
	if s.Samples[1].CacheRate != 50 {
		t.Errorf("expected CacheRate 50, got %d", s.Samples[1].CacheRate)
	}

	// Zero cacheable tokens should be skipped.
	s.AppendIfChanged(model.CacheSample{CacheRead: 0, CacheCreation: 0, InputTokens: 50})
	if len(s.Samples) != 2 {
		t.Errorf("zero-cacheable sample should be skipped, got %d samples", len(s.Samples))
	}
}

func TestState_AppendIfChanged_Threshold(t *testing.T) {
	s := &State{Samples: []model.CacheSample{}}
	baseTime := time.Now().Add(-10 * time.Minute)

	s.Samples = append(s.Samples, model.CacheSample{
		Timestamp:     baseTime,
		CacheRead:     80,
		CacheCreation: 20,
		CacheRate:     80,
	})

	// Same values but within threshold (less than 5 minutes ago) — should NOT append.
	s.Samples[0].Timestamp = time.Now().Add(-2 * time.Minute)
	s2 := &State{Samples: append([]model.CacheSample{}, s.Samples...)}
	s2.AppendIfChanged(model.CacheSample{CacheRead: 80, CacheCreation: 20})
	if len(s2.Samples) != 1 {
		t.Errorf("within threshold: should not append, got %d samples", len(s2.Samples))
	}

	// Same values but past threshold (more than 5 minutes ago) — SHOULD append.
	s3 := &State{Samples: append([]model.CacheSample{}, s.Samples...)}
	s3.Samples[0].Timestamp = time.Now().Add(-6 * time.Minute)
	s3.AppendIfChanged(model.CacheSample{CacheRead: 80, CacheCreation: 20})
	if len(s3.Samples) != 2 {
		t.Errorf("past threshold: should append, got %d samples", len(s3.Samples))
	}
}

func TestState_MaxSamplesTruncation(t *testing.T) {
	s := &State{Samples: make([]model.CacheSample, 0, maxSamples+10)}
	for i := 0; i < maxSamples+10; i++ {
		s.Samples = append(s.Samples, model.CacheSample{CacheRead: i, CacheCreation: 0, CacheRate: i})
	}

	s.AppendIfChanged(model.CacheSample{CacheRead: 999, CacheCreation: 0})
	if len(s.Samples) != maxSamples {
		t.Errorf("expected maxSamples=%d, got %d", maxSamples, len(s.Samples))
	}
	// The first retained sample should be the (10+1)th one (after truncating first 10).
	if s.Samples[0].CacheRead != 11 {
		t.Errorf("expected first sample CacheRead=11 (truncated), got CacheRead=%d", s.Samples[0].CacheRead)
	}
}
