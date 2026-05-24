package widget

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/cachestate"
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

	currentRate := cachestate.CacheHitRate(ctx.CacheRead, ctx.CacheCreation)

	parts := []string{fmt.Sprintf("cache:%d%%", currentRate)}

	if len(ctx.CacheSamples) > 0 {
		if rate5m := cachestate.RollingAverage(ctx.CacheSamples, 5*time.Minute); rate5m >= 0 {
			parts = append(parts, fmt.Sprintf("5min:%d%%", rate5m))
		}
		if rate1h := cachestate.RollingAverage(ctx.CacheSamples, 1*time.Hour); rate1h >= 0 {
			parts = append(parts, fmt.Sprintf("1h:%d%%", rate1h))
		}
	}

	plain := strings.Join(parts, " ")
	fgColor := cacheRateColor(currentRate)

	return WidgetResult{
		Text:      colorize(plain, fgColor),
		PlainText: plain,
		FgColor:   fgColor,
	}
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
