package usage

import (
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Fetch returns cached usage data, refreshing from the API when the cache
// is stale. Returns nil for API users (no OAuth credentials) or when
// credentials are unavailable. Never blocks longer than ~100ms on the fast
// path (cache hit); the HTTP call runs synchronously when the cache is stale
// because the gather stage already runs this on a background goroutine.
func Fetch(homeDir string) *model.UsageInfo {
	// Skip if using a custom API endpoint (non-Anthropic provider).
	if isUsingCustomEndpoint() {
		logging.Debug("usage: skipping — custom API endpoint configured")
		return nil
	}

	// Fast path: return cached data if fresh.
	cs := readCacheState(homeDir)
	if cs != nil && cs.fresh {
		return cs.data
	}

	// Try to acquire the lock. If busy, return stale cache (another process is fetching).
	lockStatus := tryAcquireLock(homeDir)
	if lockStatus == "busy" {
		if cs != nil {
			return cs.data
		}
		return nil
	}
	holdLock := lockStatus == "acquired"

	defer func() {
		if holdLock {
			releaseLock(homeDir)
		}
	}()

	// Re-check cache after acquiring lock (another process may have just written it).
	cs = readCacheState(homeDir)
	if cs != nil && cs.fresh {
		return cs.data
	}

	// Read credentials.
	creds := readCredentials(homeDir)
	if creds == nil {
		return nil
	}

	plan := planName(creds.SubscriptionType)
	if plan == "" {
		// API user — no usage limits.
		return nil
	}

	// Fetch from API.
	result := fetchAPI(creds.AccessToken)

	if result.data == nil {
		// API call failed.
		isRateLimited := result.err == "rate-limited"
		prevCount := 0
		if cs != nil && cs.rawFile != nil {
			prevCount = cs.rawFile.RateLimitedCount
		}

		failureData := &model.UsageInfo{
			PlanName:        plan,
			FiveHourPercent: -1,
			SevenDayPercent: -1,
			APIUnavailable:  true,
			APIError:        result.err,
		}

		opts := &writeCacheOpts{}
		if isRateLimited {
			opts.rateLimitedCount = prevCount + 1
			if result.retryAfterSec > 0 {
				opts.retryAfterUntil = time.Now().UnixMilli() + int64(result.retryAfterSec)*1000
			}

			// Preserve last good data for display during rate-limit backoff.
			var goodData *model.UsageInfo
			if cs != nil && !cs.data.APIUnavailable {
				goodData = cs.data
			} else if cs != nil && cs.rawFile != nil && cs.rawFile.LastGoodData != nil {
				goodData = cs.rawFile.LastGoodData
			}

			if goodData != nil {
				opts.lastGoodData = goodData
				writeCache(homeDir, failureData, opts)
				syncing := *goodData
				syncing.APIError = "rate-limited"
				return &syncing
			}
		}

		writeCache(homeDir, failureData, opts)
		return failureData
	}

	// Parse successful response.
	fiveHour := -1
	sevenDay := -1
	var fiveHourResetAt, sevenDayResetAt time.Time
	if result.data.FiveHour != nil {
		fiveHour = parseUtilization(result.data.FiveHour.Utilization)
		fiveHourResetAt = parseResetTime(result.data.FiveHour.ResetsAt)
	}
	if result.data.SevenDay != nil {
		sevenDay = parseUtilization(result.data.SevenDay.Utilization)
		sevenDayResetAt = parseResetTime(result.data.SevenDay.ResetsAt)
	}

	data := &model.UsageInfo{
		PlanName:        plan,
		FiveHourPercent: fiveHour,
		FiveHourResetAt: fiveHourResetAt,
		SevenDayPercent: sevenDay,
		SevenDayResetAt: sevenDayResetAt,
	}

	writeCache(homeDir, data, &writeCacheOpts{lastGoodData: data})
	return data
}
