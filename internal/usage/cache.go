package usage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
)

const (
	cacheTTL             = 180 * time.Second // 3 minutes for successful data
	cacheFailureTTL      = 15 * time.Second  // 15 seconds for failed requests
	rateLimitBaseBackoff = 60 * time.Second  // 60s base for 429 backoff
	rateLimitMaxBackoff  = 5 * time.Minute   // 5 min max backoff
	lockStaleThreshold   = 30 * time.Second  // stale lock cleanup
)

// cacheFile is the on-disk JSON shape.
type cacheFile struct {
	Data             *UsageData `json:"data"`
	Timestamp        int64      `json:"timestamp"`          // Unix ms
	RateLimitedCount int        `json:"rate_limited_count"` // for exponential backoff
	RetryAfterUntil  int64      `json:"retry_after_until"`  // absolute ms
	LastGoodData     *UsageData `json:"last_good_data"`     // preserved across rate-limit periods
}

// cacheState is the result of reading and evaluating the cache file.
type cacheState struct {
	data    *UsageData
	fresh   bool
	rawFile *cacheFile // the full cache file for rate-limit bookkeeping
}

func cachePath(homeDir string) string {
	return filepath.Join(pluginDir(homeDir), "usage-cache.json")
}

func cacheLockPath(homeDir string) string {
	return filepath.Join(pluginDir(homeDir), "usage-cache.lock")
}

// readCacheState reads and evaluates the cache file. Returns nil when the
// file is missing or corrupt.
func readCacheState(homeDir string) *cacheState {
	data, err := os.ReadFile(cachePath(homeDir))
	if err != nil {
		return nil
	}

	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil
	}
	if cf.Data == nil {
		return nil
	}

	now := time.Now().UnixMilli()

	// During rate-limit backoff, serve lastGoodData with a syncing hint
	// so the user sees their most recent real numbers instead of an error.
	displayData := cf.Data
	if cf.Data.APIError == "rate-limited" && cf.LastGoodData != nil {
		displayData = cf.LastGoodData.clone()
		displayData.APIError = "rate-limited" // syncing hint
	}

	// Check rate-limit backoff: if we're still in the backoff window, treat as fresh.
	if retryUntil := rateLimitedRetryUntil(&cf); retryUntil > 0 && now < retryUntil {
		return &cacheState{data: displayData, fresh: true, rawFile: &cf}
	}

	// Determine freshness based on success/failure TTL.
	ttl := cacheTTL
	if cf.Data.APIUnavailable {
		ttl = cacheFailureTTL
	}
	fresh := time.Duration(now-cf.Timestamp)*time.Millisecond < ttl

	return &cacheState{data: displayData, fresh: fresh, rawFile: &cf}
}

// rateLimitedRetryUntil computes the absolute timestamp (ms) when the next
// retry is allowed, considering both Retry-After headers and exponential backoff.
func rateLimitedRetryUntil(cf *cacheFile) int64 {
	if cf.Data == nil || cf.Data.APIError != "rate-limited" {
		return 0
	}

	// Prefer server-specified Retry-After.
	if cf.RetryAfterUntil > cf.Timestamp {
		return cf.RetryAfterUntil
	}

	// Exponential backoff: 60s, 120s, 240s, capped at 5 min.
	if cf.RateLimitedCount > 0 {
		backoff := rateLimitBaseBackoff * time.Duration(math.Pow(2, float64(cf.RateLimitedCount-1)))
		if backoff > rateLimitMaxBackoff {
			backoff = rateLimitMaxBackoff
		}
		return cf.Timestamp + backoff.Milliseconds()
	}

	return 0
}

// writeCache writes the cache file atomically.
func writeCache(homeDir string, data *UsageData, opts *writeCacheOpts) {
	dir := pluginDir(homeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	cf := cacheFile{
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
	if opts != nil {
		cf.RateLimitedCount = opts.rateLimitedCount
		cf.RetryAfterUntil = opts.retryAfterUntil
		cf.LastGoodData = opts.lastGoodData
	}

	out, err := json.Marshal(cf)
	if err != nil {
		return
	}
	_ = os.WriteFile(cachePath(homeDir), out, 0o644)
}

type writeCacheOpts struct {
	rateLimitedCount int
	retryAfterUntil  int64
	lastGoodData     *UsageData
}

// tryAcquireLock attempts to acquire the cache lock using exclusive file creation.
// Returns "acquired", "busy", or "unsupported".
func tryAcquireLock(homeDir string) string {
	lockPath := cacheLockPath(homeDir)
	dir := filepath.Dir(lockPath)
	_ = os.MkdirAll(dir, 0o755)

	// O_CREATE|O_EXCL is atomic: succeeds only if the file doesn't exist.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists — check if it's stale.
			return checkStaleLock(homeDir)
		}
		logging.Debug("usage: lock unavailable: %v", err)
		return "unsupported"
	}
	// Write timestamp so we can detect stale locks.
	fmt.Fprintf(f, "%d", time.Now().UnixMilli())
	f.Close()
	return "acquired"
}

// checkStaleLock checks if an existing lock file is stale and cleans it up.
func checkStaleLock(homeDir string) string {
	lockPath := cacheLockPath(homeDir)

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return "busy"
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		// Unparseable — check file modtime.
		info, err := os.Stat(lockPath)
		if err != nil || time.Since(info.ModTime()) < lockStaleThreshold {
			return "busy"
		}
	} else if time.Duration(time.Now().UnixMilli()-ts)*time.Millisecond < lockStaleThreshold {
		return "busy"
	}

	// Stale lock — remove and retry.
	if os.Remove(lockPath) != nil {
		return "busy"
	}
	return tryAcquireLock(homeDir)
}

// releaseLock removes the cache lock file.
func releaseLock(homeDir string) {
	_ = os.Remove(cacheLockPath(homeDir))
}
