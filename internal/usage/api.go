package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
)

const (
	usageAPIURL     = "https://api.anthropic.com/api/oauth/usage"
	usageAPITimeout = 15 * time.Second
	usageAPIBeta    = "oauth-2025-04-20"
	userAgent       = "tail-claude-hud/1.0"
)

// apiResponse is the raw JSON shape returned by the usage API.
type apiResponse struct {
	FiveHour *apiWindow `json:"five_hour"`
	SevenDay *apiWindow `json:"seven_day"`
}

// apiWindow is a single rate-limit window from the API.
type apiWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

// apiResult wraps the parsed API response or an error descriptor.
type apiResult struct {
	data          *apiResponse
	err           string // "rate-limited", "http-NNN", "network", "timeout", "parse", ""
	retryAfterSec int    // from Retry-After header on 429
}

// fetchAPI calls the Anthropic OAuth usage endpoint and returns the result.
// The call has a 15-second timeout. Errors are returned as descriptive strings
// rather than Go errors so the cache can persist them directly.
func fetchAPI(accessToken string) *apiResult {
	client := &http.Client{Timeout: usageAPITimeout}

	req, err := http.NewRequest("GET", usageAPIURL, nil)
	if err != nil {
		return &apiResult{err: "network"}
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", usageAPIBeta)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			return &apiResult{err: "timeout"}
		}
		return &apiResult{err: "network"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &apiResult{err: "network"}
	}

	if resp.StatusCode != 200 {
		logging.Debug("usage: API returned %d", resp.StatusCode)
		if resp.StatusCode == 429 {
			retryAfter := parseRetryAfterSeconds(resp.Header.Get("Retry-After"))
			return &apiResult{err: "rate-limited", retryAfterSec: retryAfter}
		}
		return &apiResult{err: fmt.Sprintf("http-%d", resp.StatusCode)}
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		logging.Debug("usage: failed to parse API response: %v", err)
		return &apiResult{err: "parse"}
	}

	return &apiResult{data: &parsed}
}

// parseRetryAfterSeconds parses a Retry-After header value as either
// an integer (seconds) or an HTTP-date. Returns 0 when unparseable.
func parseRetryAfterSeconds(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	// Try integer seconds first.
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return secs
	}

	// Try HTTP-date.
	if t, err := time.Parse(time.RFC1123, raw); err == nil {
		secs := int(time.Until(t).Seconds())
		if secs > 0 {
			return secs
		}
	}

	return 0
}

// parseUtilization clamps a utilization value to 0-100 and rounds it.
// Returns -1 when the value is nil or non-finite.
func parseUtilization(v *float64) int {
	if v == nil {
		return -1
	}
	val := *v
	if val != val { // NaN check
		return -1
	}
	if val < 0 {
		return 0
	}
	if val > 100 {
		return 100
	}
	return int(val + 0.5) // round
}

// parseResetTime parses an ISO 8601 date string into a time.Time.
// Returns zero time when the string is nil or unparseable.
func parseResetTime(s *string) time.Time {
	if s == nil || *s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		// Try with fractional seconds.
		t, err = time.Parse("2006-01-02T15:04:05.999Z07:00", *s)
		if err != nil {
			logging.Debug("usage: invalid reset time: %s", *s)
			return time.Time{}
		}
	}
	return t
}

// isUsingCustomEndpoint returns true when ANTHROPIC_BASE_URL points to a
// non-Anthropic host, meaning the OAuth usage API is not applicable.
func isUsingCustomEndpoint() bool {
	baseURL := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE_URL"))
	}
	if baseURL == "" {
		return false
	}
	// Accept any URL that points to api.anthropic.com.
	return !strings.HasPrefix(baseURL, "https://api.anthropic.com")
}
