package widget

import (
	"fmt"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Speed renders a rolling tokens/sec average from transcript token samples.
//
// The window is defined by cfg.Speed.WindowSecs (default 30s). Only samples
// whose timestamps fall within the last WindowSecs seconds are counted.
// When WindowSecs is <= 0 the full session history is used.
//
// Format: "1.2k tok/s" or "" when no data is available.
func Speed(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Transcript == nil || len(ctx.Transcript.TokenSamples) == 0 {
		return ""
	}

	samples := ctx.Transcript.TokenSamples
	windowSecs := cfg.Speed.WindowSecs
	if windowSecs <= 0 {
		// Use session average: span from first sample to last.
		return computeSpeedOverSamples(samples)
	}

	// Find the latest timestamp to anchor the window end.
	latestTS := samples[len(samples)-1].Timestamp
	for _, s := range samples {
		if s.Timestamp.After(latestTS) {
			latestTS = s.Timestamp
		}
	}

	windowStart := latestTS.Add(-time.Duration(windowSecs) * time.Second)

	// Collect samples within the window.
	var windowed []model.TokenSample
	for _, s := range samples {
		if !s.Timestamp.Before(windowStart) {
			windowed = append(windowed, s)
		}
	}

	if len(windowed) == 0 {
		return ""
	}

	return computeSpeedOverSamples(windowed)
}

// computeSpeedOverSamples calculates tokens/sec for a slice of samples.
// Returns "" when the samples don't cover a measurable time span.
func computeSpeedOverSamples(samples []model.TokenSample) string {
	if len(samples) == 0 {
		return ""
	}

	var totalTokens int
	earliest := samples[0].Timestamp
	latest := samples[0].Timestamp

	for _, s := range samples {
		totalTokens += s.Tokens
		if s.Timestamp.Before(earliest) {
			earliest = s.Timestamp
		}
		if s.Timestamp.After(latest) {
			latest = s.Timestamp
		}
	}

	elapsed := latest.Sub(earliest).Seconds()
	// Need at least a small time span to compute a meaningful rate.
	// With a single sample there's no elapsed time; return "" to avoid
	// divide-by-zero or misleadingly large rates.
	if elapsed < 0.001 {
		return ""
	}

	tokPerSec := float64(totalTokens) / elapsed
	return fmt.Sprintf("%s tok/s", formatTokenCount(int(tokPerSec)))
}
