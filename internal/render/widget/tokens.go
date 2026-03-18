package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Tokens renders token count and cache hit ratio from the most recent API call.
// Format: "126k tok 98% cached"
//
// The current_usage fields from stdin are per-call snapshots (not session totals):
//   - input_tokens: uncacheable tail after the last cache breakpoint
//   - cache_creation_input_tokens: tokens written to a new cache entry
//   - cache_read_input_tokens: tokens served from an existing cache
//
// The cache ratio is cache_read / (cache_read + cache_creation). A high ratio
// means the prompt cache is healthy. A drop signals a cache bust (prompt
// structure changed enough to invalidate the cache). The uncacheable
// input_tokens are excluded from the ratio since they can't be cached
// regardless.
//
// Returns an empty WidgetResult when all token counts are zero.
func Tokens(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	uncached := ctx.InputTokens
	cacheCreate := ctx.CacheCreation
	cacheRead := ctx.CacheRead

	if uncached == 0 && cacheCreate == 0 && cacheRead == 0 {
		return WidgetResult{}
	}

	total := uncached + cacheCreate + cacheRead
	plain := fmt.Sprintf("%s tok", formatTokenCount(total))

	// Cache ratio: what fraction of cacheable tokens were served from cache.
	// Only meaningful when there are cacheable tokens (creation + read > 0).
	cacheable := cacheCreate + cacheRead
	if cacheable > 0 {
		cachePercent := (cacheRead * 100) / cacheable
		plain = fmt.Sprintf("%s %d%% cached", plain, cachePercent)
	}

	return WidgetResult{
		Text:      MutedStyle.Render(plain),
		PlainText: plain,
		FgColor:   "8",
	}
}
