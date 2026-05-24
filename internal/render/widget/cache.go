package widget

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Cache renders cache hit-rate metrics from the most recent API call and
// rolling averages over 5-minute and 1-hour windows.
//
// Format: "cache:X 5min:X 1h:X"
//
// Color coding based on the current cache hit rate:
//   - < 40: red (alert — cache is underutilised)
//   - 40–60: yellow (moderate)
//   - > 60: white/default (healthy)
//
// The 5min and 1h fields are only meaningful when the model is a
// Claude-series model that supports prompt caching.
func Cache(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.CacheRead == 0 && ctx.CacheCreation == 0 {
		return WidgetResult{}
	}

	currentRate := cacheRate(ctx.CacheRead, ctx.CacheCreation)

	parts := []string{fmt.Sprintf("cache:%d%%", currentRate)}

	if len(ctx.CacheSamples) > 0 {
		if rate5m := rollingCacheRate(ctx.CacheSamples, 5*time.Minute); rate5m >= 0 {
			parts = append(parts, fmt.Sprintf("5min:%d%%", rate5m))
		}
		if rate1h := rollingCacheRate(ctx.CacheSamples, 1*time.Hour); rate1h >= 0 {
			parts = append(parts, fmt.Sprintf("1h:%d%%", rate1h))
		}
	}

	plain := joinParts(parts)
	fgColor := cacheRateColor(currentRate)

	return WidgetResult{
		Text:      colorize(plain, fgColor),
		PlainText: plain,
		FgColor:   fgColor,
	}
}

// cacheRate computes the cache hit rate percentage.
// Returns 0 when there are no cacheable tokens.
func cacheRate(cacheRead, cacheCreation int) int {
	cacheable := cacheRead + cacheCreation
	if cacheable <= 0 {
		return 0
	}
	return (cacheRead * 100) / cacheable
}

// rollingCacheRate computes the average cache hit rate over the given window.
// Returns -1 when no samples fall within the window.
func rollingCacheRate(samples []model.CacheSample, window time.Duration) int {
	cutoff := time.Now().Add(-window)
	var windowed []model.CacheSample
	for _, s := range samples {
		if !s.Timestamp.Before(cutoff) {
			windowed = append(windowed, s)
		}
	}

	if len(windowed) == 0 {
		return -1
	}

	total := 0
	for _, s := range windowed {
		total += s.CacheRate
	}
	return total / len(windowed)
}

// joinParts joins cache display parts with a space separator.
func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

// cacheRateColor returns the ANSI color string for a cache hit rate.
//   - < 40: "9" (red)
//   - 40–60: "11" (yellow)
//   - > 60: "" (default/white)
func cacheRateColor(rate int) string {
	switch {
	case rate < 40:
		return "9" // red
	case rate <= 60:
		return "11" // yellow
	default:
		return "" // default/white
	}
}

// colorize applies the given ANSI color to the text.
func colorize(text, color string) string {
	if color == "" {
		return text
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(text)
}
