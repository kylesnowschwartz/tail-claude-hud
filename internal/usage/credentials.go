// Package usage fetches and caches Anthropic OAuth rate-limit data.
//
// Claude Code plan subscribers (Pro, Max, Team) have rolling 5-hour and 7-day
// usage windows. This package retrieves that data from the Anthropic OAuth API,
// caches it to disk so the statusline's ~300ms invocations don't block on HTTP,
// and exposes a single Fetch() entry point for the gather stage.
package usage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
)

const (
	keychainServiceName = "Claude Code-credentials"
	keychainTimeoutMs   = 3000
	keychainBackoffMs   = 60_000
)

// credentials holds the OAuth token and plan type needed for the usage API.
type credentials struct {
	AccessToken      string
	SubscriptionType string
}

// readCredentials tries macOS Keychain first, then falls back to the
// file at {configDir}/.credentials.json. Returns nil when no valid
// credentials are available.
func readCredentials(homeDir string) *credentials {
	configDir := claudeConfigDir(homeDir)

	// Try macOS Keychain first.
	if runtime.GOOS == "darwin" {
		if creds := readKeychainCredentials(configDir, homeDir); creds != nil {
			// Supplement missing subscriptionType from file.
			if creds.SubscriptionType == "" {
				if fileSub := readFileSubscriptionType(configDir); fileSub != "" {
					creds.SubscriptionType = fileSub
				}
			}
			return creds
		}
	}

	// Fall back to file-based credentials.
	return readFileCredentials(configDir)
}

// claudeConfigDir returns the Claude Code config directory, respecting
// CLAUDE_CONFIG_DIR if set.
func claudeConfigDir(homeDir string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(homeDir, ".claude")
}

// readKeychainCredentials reads OAuth credentials from the macOS Keychain.
// Uses /usr/bin/security with a 3-second timeout. Returns nil when the
// keychain item is missing, expired, or the keychain is in a backoff period.
func readKeychainCredentials(configDir, homeDir string) *credentials {
	if isKeychainBackoff(homeDir) {
		logging.Debug("usage: keychain in backoff period, skipping")
		return nil
	}

	serviceNames := keychainServiceNames(configDir, homeDir)
	accountName := keychainAccountName()

	// Try with account name first, then without.
	for _, tryAccount := range []bool{true, false} {
		for _, svc := range serviceNames {
			var args []string
			if tryAccount && accountName != "" {
				args = []string{"find-generic-password", "-s", svc, "-a", accountName, "-w"}
			} else if !tryAccount {
				args = []string{"find-generic-password", "-s", svc, "-w"}
			} else {
				continue
			}

			creds, err := execKeychain(args)
			if err != nil {
				// "could not be found" is expected — not a backoff-worthy failure.
				if isMissingKeychainError(err) {
					continue
				}
				logging.Debug("usage: keychain error for service %s: %v", svc, err)
				recordKeychainFailure(homeDir)
				continue
			}
			if creds != nil {
				return creds
			}
		}
	}

	return nil
}

// execKeychain runs /usr/bin/security with the given args and parses the JSON output.
func execKeychain(args []string) (*credentials, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(keychainTimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	return parseCredentialsJSON([]byte(trimmed), time.Now().UnixMilli())
}

// credentialsFile mirrors the JSON shape of ~/.claude/.credentials.json.
type credentialsFile struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		RefreshToken     string `json:"refreshToken"`
		SubscriptionType string `json:"subscriptionType"`
		ExpiresAt        *int64 `json:"expiresAt"` // Unix milliseconds
	} `json:"claudeAiOauth"`
}

// parseCredentialsJSON extracts access token and subscription type from
// the Claude credentials JSON blob. Returns nil when the token is missing
// or expired.
func parseCredentialsJSON(data []byte, nowMs int64) (*credentials, error) {
	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	if cf.ClaudeAiOauth == nil || cf.ClaudeAiOauth.AccessToken == "" {
		return nil, nil
	}
	// Check expiry.
	if cf.ClaudeAiOauth.ExpiresAt != nil && *cf.ClaudeAiOauth.ExpiresAt <= nowMs {
		logging.Debug("usage: OAuth token expired")
		return nil, nil
	}
	return &credentials{
		AccessToken:      cf.ClaudeAiOauth.AccessToken,
		SubscriptionType: cf.ClaudeAiOauth.SubscriptionType,
	}, nil
}

// readFileCredentials reads credentials from {configDir}/.credentials.json.
func readFileCredentials(configDir string) *credentials {
	path := filepath.Join(configDir, ".credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	creds, err := parseCredentialsJSON(data, time.Now().UnixMilli())
	if err != nil {
		logging.Debug("usage: failed to parse credentials file: %v", err)
		return nil
	}
	return creds
}

// readFileSubscriptionType reads only the subscriptionType from the file.
func readFileSubscriptionType(configDir string) string {
	path := filepath.Join(configDir, ".credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cf credentialsFile
	if json.Unmarshal(data, &cf) != nil || cf.ClaudeAiOauth == nil {
		return ""
	}
	return strings.TrimSpace(cf.ClaudeAiOauth.SubscriptionType)
}

// keychainServiceNames returns the list of macOS Keychain service names to
// try, in order. Claude Code uses the default service for ~/.claude and a
// hashed suffix for custom config directories.
func keychainServiceNames(configDir, homeDir string) []string {
	defaultDir := filepath.Join(homeDir, ".claude")
	normalConfig := filepath.Clean(configDir)
	normalDefault := filepath.Clean(defaultDir)

	names := []string{}
	if normalConfig == normalDefault {
		names = append(names, keychainServiceName)
	} else {
		hash := sha256.Sum256([]byte(normalConfig))
		names = append(names, fmt.Sprintf("%s-%x", keychainServiceName, hash[:4]))
	}

	// Also check CLAUDE_CONFIG_DIR if set and different.
	if envDir := os.Getenv("CLAUDE_CONFIG_DIR"); envDir != "" {
		normalEnv := filepath.Clean(envDir)
		if normalEnv == normalDefault {
			names = append(names, keychainServiceName)
		} else {
			hash := sha256.Sum256([]byte(envDir))
			names = append(names, fmt.Sprintf("%s-%x", keychainServiceName, hash[:4]))
		}
	}

	// Always try the legacy name last.
	names = append(names, keychainServiceName)

	// Deduplicate.
	seen := make(map[string]bool, len(names))
	deduped := names[:0]
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			deduped = append(deduped, n)
		}
	}
	return deduped
}

// keychainAccountName returns the current OS username for keychain lookups.
func keychainAccountName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Username)
}

// isMissingKeychainError checks if a keychain error means the item was not found.
func isMissingKeychainError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not be found") ||
		strings.Contains(msg, "exit status 44")
}

// keychainBackoffPath returns the path for the keychain failure backoff file.
func keychainBackoffPath(homeDir string) string {
	return filepath.Join(pluginDir(homeDir), ".keychain-backoff")
}

// isKeychainBackoff returns true when a recent keychain failure was recorded.
func isKeychainBackoff(homeDir string) bool {
	data, err := os.ReadFile(keychainBackoffPath(homeDir))
	if err != nil {
		return false
	}
	var ts int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &ts); err != nil {
		return false
	}
	return time.Now().UnixMilli()-ts < keychainBackoffMs
}

// recordKeychainFailure writes the current timestamp to the backoff file.
func recordKeychainFailure(homeDir string) {
	path := keychainBackoffPath(homeDir)
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0o644)
}

// pluginDir returns the plugin state directory for a given home.
func pluginDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "plugins", "tail-claude-hud")
}

// planName maps a subscriptionType string to a display name.
// Returns "" for API users (no usage limits to show).
func planName(subscriptionType string) string {
	lower := strings.ToLower(subscriptionType)
	switch {
	case strings.Contains(lower, "max"):
		return "Max"
	case strings.Contains(lower, "pro"):
		return "Pro"
	case strings.Contains(lower, "team"):
		return "Team"
	case subscriptionType == "" || strings.Contains(lower, "api"):
		return ""
	default:
		if len(subscriptionType) > 0 {
			return strings.ToUpper(subscriptionType[:1]) + subscriptionType[1:]
		}
		return ""
	}
}
