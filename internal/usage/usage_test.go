package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCredentialsJSON_ValidToken(t *testing.T) {
	data := `{"claudeAiOauth":{"accessToken":"tok-123","subscriptionType":"pro_plan"}}`
	creds, err := parseCredentialsJSON([]byte(data), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected credentials, got nil")
	}
	if creds.AccessToken != "tok-123" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "tok-123")
	}
	if creds.SubscriptionType != "pro_plan" {
		t.Errorf("SubscriptionType = %q, want %q", creds.SubscriptionType, "pro_plan")
	}
}

func TestParseCredentialsJSON_ExpiredToken(t *testing.T) {
	expires := time.Now().Add(-1 * time.Hour).UnixMilli()
	data := `{"claudeAiOauth":{"accessToken":"tok-123","expiresAt":` + itoa(expires) + `}}`
	creds, err := parseCredentialsJSON([]byte(data), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil for expired token, got credentials")
	}
}

func TestParseCredentialsJSON_MissingToken(t *testing.T) {
	data := `{"claudeAiOauth":{"subscriptionType":"pro_plan"}}`
	creds, err := parseCredentialsJSON([]byte(data), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil for missing token, got credentials")
	}
}

func TestParseCredentialsJSON_InvalidJSON(t *testing.T) {
	_, err := parseCredentialsJSON([]byte("not json"), time.Now().UnixMilli())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPlanName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pro_plan", "Pro"},
		{"max_plan", "Max"},
		{"team_plan", "Team"},
		{"", ""},
		{"api", ""},
		{"enterprise", "Enterprise"},
	}
	for _, tt := range tests {
		got := planName(tt.input)
		if got != tt.want {
			t.Errorf("planName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReadFileCredentials(t *testing.T) {
	dir := t.TempDir()
	data := `{"claudeAiOauth":{"accessToken":"file-tok","subscriptionType":"max_plan"}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	creds := readFileCredentials(dir)
	if creds == nil {
		t.Fatal("expected credentials from file, got nil")
	}
	if creds.AccessToken != "file-tok" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "file-tok")
	}
}

func TestReadFileCredentials_MissingFile(t *testing.T) {
	creds := readFileCredentials(t.TempDir())
	if creds != nil {
		t.Error("expected nil for missing file")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	// Create the plugin dir structure.
	if err := os.MkdirAll(pluginDir(home), 0o755); err != nil {
		t.Fatal(err)
	}

	data := &UsageData{
		PlanName:        "Pro",
		FiveHourPercent: 42,
		SevenDayPercent: 60,
	}

	writeCache(home, data, nil)

	cs := readCacheState(home)
	if cs == nil {
		t.Fatal("expected cache state, got nil")
	}
	if !cs.fresh {
		t.Error("expected fresh cache")
	}
	if cs.data.FiveHourPercent != 42 {
		t.Errorf("FiveHourPercent = %d, want 42", cs.data.FiveHourPercent)
	}
	if cs.data.SevenDayPercent != 60 {
		t.Errorf("SevenDayPercent = %d, want 60", cs.data.SevenDayPercent)
	}
}

func TestCacheStaleAfterTTL(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(pluginDir(home), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a cache entry with an old timestamp.
	cf := cacheFile{
		Data: &UsageData{
			PlanName:        "Pro",
			FiveHourPercent: 42,
		},
		Timestamp: time.Now().Add(-5 * time.Minute).UnixMilli(), // well past TTL
	}
	out, _ := json.Marshal(cf)
	if err := os.WriteFile(cachePath(home), out, 0o644); err != nil {
		t.Fatal(err)
	}

	cs := readCacheState(home)
	if cs == nil {
		t.Fatal("expected cache state, got nil")
	}
	if cs.fresh {
		t.Error("expected stale cache")
	}
}

func TestCacheLock(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(pluginDir(home), 0o755); err != nil {
		t.Fatal(err)
	}

	status := tryAcquireLock(home)
	if status != "acquired" {
		t.Fatalf("first lock = %q, want 'acquired'", status)
	}

	status = tryAcquireLock(home)
	if status != "busy" {
		t.Fatalf("second lock = %q, want 'busy'", status)
	}

	releaseLock(home)

	status = tryAcquireLock(home)
	if status != "acquired" {
		t.Fatalf("after release = %q, want 'acquired'", status)
	}
	releaseLock(home)
}

func TestParseUtilization(t *testing.T) {
	tests := []struct {
		input *float64
		want  int
	}{
		{nil, -1},
		{floatPtr(0), 0},
		{floatPtr(42.3), 42},
		{floatPtr(42.7), 43},
		{floatPtr(100), 100},
		{floatPtr(150), 100},
		{floatPtr(-5), 0},
	}
	for _, tt := range tests {
		got := parseUtilization(tt.input)
		if got != tt.want {
			if tt.input != nil {
				t.Errorf("parseUtilization(%v) = %d, want %d", *tt.input, got, tt.want)
			} else {
				t.Errorf("parseUtilization(nil) = %d, want %d", got, tt.want)
			}
		}
	}
}

func TestParseResetTime(t *testing.T) {
	// Valid RFC3339.
	s := "2026-03-20T10:00:00Z"
	got := parseResetTime(&s)
	if got.IsZero() {
		t.Error("expected non-zero time for valid RFC3339")
	}

	// Nil.
	got = parseResetTime(nil)
	if !got.IsZero() {
		t.Error("expected zero time for nil")
	}

	// Invalid.
	bad := "not-a-date"
	got = parseResetTime(&bad)
	if !got.IsZero() {
		t.Error("expected zero time for invalid string")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"60", 60},
		{"0", 0},
		{"", 0},
		{"not-a-number", 0},
	}
	for _, tt := range tests {
		got := parseRetryAfterSeconds(tt.input)
		if got != tt.want {
			t.Errorf("parseRetryAfterSeconds(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCacheRateLimitedBackoff(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(pluginDir(home), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a rate-limited cache entry that is "fresh" due to backoff.
	lastGood := &UsageData{
		PlanName:        "Pro",
		FiveHourPercent: 25,
		SevenDayPercent: 40,
	}
	cf := cacheFile{
		Data: &UsageData{
			PlanName:       "Pro",
			APIUnavailable: true,
			APIError:       "rate-limited",
		},
		Timestamp:        time.Now().UnixMilli(),
		RateLimitedCount: 1,
		LastGoodData:     lastGood,
	}
	out, _ := json.Marshal(cf)
	_ = os.WriteFile(cachePath(home), out, 0o644)

	cs := readCacheState(home)
	if cs == nil {
		t.Fatal("expected cache state")
	}
	if !cs.fresh {
		t.Error("expected fresh (within backoff window)")
	}
	// Should show last good data with syncing hint.
	if cs.data.FiveHourPercent != 25 {
		t.Errorf("FiveHourPercent = %d, want 25", cs.data.FiveHourPercent)
	}
	if cs.data.APIError != "rate-limited" {
		t.Errorf("APIError = %q, want 'rate-limited'", cs.data.APIError)
	}
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}

func floatPtr(f float64) *float64 { return &f }
