package widget

import (
	"fmt"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Trend arrow ratios: the recent half-window must lead or lag the older half
// by this much before an arrow is shown, so steady generation stays arrowless.
const (
	trendUpRatio   = 1.25
	trendDownRatio = 0.75
)

// Speed renders a rolling tokens/sec average from transcript token samples,
// with a trend arrow when generation is ramping up or slowing down.
//
// The window is defined by cfg.Speed.WindowSecs (default 30s). Tokens from
// samples within the last WindowSecs seconds of wall-clock time are averaged
// over the whole window — a fixed denominator, so the rate moves smoothly and
// decays to zero as samples age out instead of spiking on clustered samples.
// The widget hides once generation has been idle past the window. When
// WindowSecs is <= 0 the full session history is used (no arrow).
//
// Format: "1.2k tok/s ↑" or empty when no data is available.
// Returns an empty WidgetResult when ctx.Transcript is nil or no token samples exist.
func Speed(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Transcript == nil || len(ctx.Transcript.TokenSamples) == 0 {
		return WidgetResult{}
	}

	samples := ctx.Transcript.TokenSamples
	windowSecs := cfg.Speed.WindowSecs
	if windowSecs <= 0 {
		// Use session average: span from first sample to last.
		text := computeSpeedOverSamples(samples)
		if text == "" {
			return WidgetResult{}
		}
		return WidgetResult{
			Text:      MutedStyle.Render(text),
			PlainText: text,
			FgColor:   "8",
		}
	}

	// Anchor the window end at wall-clock now, not at the newest sample:
	// a sample-anchored window slides with the data and would keep showing
	// the last burst's rate indefinitely after generation stops.
	window := time.Duration(windowSecs) * time.Second
	now := time.Now()
	windowStart := now.Add(-window)
	midpoint := now.Add(-window / 2)

	var total, older, recent int
	for _, s := range samples {
		if s.Timestamp.Before(windowStart) {
			continue
		}
		total += s.Tokens
		if s.Timestamp.Before(midpoint) {
			older += s.Tokens
		} else {
			recent += s.Tokens
		}
	}

	if total == 0 {
		return WidgetResult{}
	}

	rate := float64(total) / float64(windowSecs)
	text := fmt.Sprintf("%s tok/s", formatTokenCount(int(rate)))
	if arrow := trendArrow(recent, older); arrow != "" {
		text += " " + arrow
	}
	return WidgetResult{
		Text:      MutedStyle.Render(text),
		PlainText: text,
		FgColor:   "8",
	}
}

// trendArrow compares token volume in the two halves of the window. The
// halves span equal time, so raw sums compare directly: "↑" when the recent
// half leads by trendUpRatio, "↓" when it trails below trendDownRatio, and
// "" when roughly steady.
func trendArrow(recent, older int) string {
	switch {
	case float64(recent) > float64(older)*trendUpRatio:
		return "↑"
	case float64(recent) < float64(older)*trendDownRatio:
		return "↓"
	default:
		return ""
	}
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
