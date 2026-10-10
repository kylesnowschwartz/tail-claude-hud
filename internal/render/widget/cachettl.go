package widget

import (
	"fmt"
	"math"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/color"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Prompt-cache lifetimes the API writes with.
const (
	cacheTTL5m = 5 * time.Minute
	cacheTTL1h = time.Hour
)

// defaultCacheTTL is the lifetime Claude Code requests on the main thread of
// a subscription that is not in overage; it applies until the transcript
// reports a cache write.
const defaultCacheTTL = cacheTTL1h

// cacheWarnFraction is the share of the TTL left at which the countdown
// switches to the warning color.
const cacheWarnFraction = 0.2

// cacheRingGlyphs drain from full to nearly empty as the cache cools;
// cacheColdGlyph replaces them once the TTL has lapsed.
var cacheRingGlyphs = [...]string{"●", "◕", "◑", "◔", "○"}

const cacheColdGlyph = "◌"

// CacheTTL renders how long the main conversation's prompt cache stays warm:
// a ring glyph that empties as time runs out, plus the time left ("47m",
// "0:42" in the last minute, "Cold" once lapsed). Once cold, the next message
// writes the whole conversation to the cache again.
//
// The countdown starts at Transcript.LastRequestAt and runs for the TTL
// chosen by cfg.CacheTTL.TTL ("auto" follows the transcript's latest cache
// write, defaulting to 1h). Returns empty before any answered request.
func CacheTTL(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	return cacheTTLAt(ctx, cfg, time.Now())
}

// cacheTTLAt renders the countdown as of now.
func cacheTTLAt(ctx *model.RenderContext, cfg *config.Config, now time.Time) WidgetResult {
	if ctx.Transcript == nil || ctx.Transcript.LastRequestAt.IsZero() {
		return WidgetResult{}
	}

	ttl := effectiveCacheTTL(cfg.CacheTTL.TTL, ctx.Transcript.CacheTTL)
	left := ctx.Transcript.LastRequestAt.Add(ttl).Sub(now)

	if left <= 0 {
		text := cacheColdGlyph + " Cold"
		return WidgetResult{Text: MutedStyle.Render(text), PlainText: text, FgColor: "8"}
	}

	fraction := float64(left) / float64(ttl)
	text := cacheRingGlyph(fraction) + " " + cacheTimeLeft(left)
	if fraction > cacheWarnFraction {
		return WidgetResult{Text: MutedStyle.Render(text), PlainText: text, FgColor: "8"}
	}

	warnFg := "3"
	if cfg.Style.Colors.Warning != "" {
		warnFg = color.ResolveColorName(cfg.Style.Colors.Warning)
	}
	return WidgetResult{
		Text:      colorStyle(cfg.Style.Colors.Warning, yellowStyle).Render(text),
		PlainText: text,
		FgColor:   warnFg,
	}
}

// effectiveCacheTTL picks the lifetime the countdown runs against: a forced
// "5m"/"1h" setting, else the transcript's latest write, else the default.
func effectiveCacheTTL(setting string, observed time.Duration) time.Duration {
	switch setting {
	case "5m":
		return cacheTTL5m
	case "1h":
		return cacheTTL1h
	}
	if observed > 0 {
		return observed
	}
	return defaultCacheTTL
}

// cacheRingGlyph maps the remaining fraction of the TTL (0, 1] to the ring
// glyph whose fill is nearest.
func cacheRingGlyph(fraction float64) string {
	last := len(cacheRingGlyphs) - 1
	idx := int(math.Round((1 - fraction) * float64(last)))
	return cacheRingGlyphs[max(0, min(last, idx))]
}

// cacheTimeLeft formats a positive duration as whole minutes rounded up
// ("47m"), or as "0:SS" seconds rounded up in the last minute. A remainder
// that rounds up to a full minute reads "1m", never "0:60".
func cacheTimeLeft(left time.Duration) string {
	if secs := int(math.Ceil(left.Seconds())); secs < 60 {
		return fmt.Sprintf("0:%02d", secs)
	}
	return fmt.Sprintf("%dm", int(math.Ceil(left.Minutes())))
}
