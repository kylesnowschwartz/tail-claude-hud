// Package extracmd executes an optional user-configured command and returns its
// label output for display in the statusline.
package extracmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const timeout = 3 * time.Second

// ansiColorRE matches ANSI SGR (Select Graphic Rendition) escape sequences:
// ESC [ <digits and semicolons> m
// e.g. \x1b[0m, \x1b[31m, \x1b[1;32m
var ansiColorRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// nonColorEscapeRE matches escape sequences that are NOT ANSI SGR color codes.
// After color escapes are replaced with placeholders, this strips:
//   - CSI sequences: ESC [ <param bytes> <intermediate bytes> <final byte A-Za-z @-~>
//     e.g. \x1b[2J (clear screen), \x1b[H (cursor home)
//   - OSC sequences: ESC ] ... BEL or ESC \
//     e.g. \x1b]0;title\x07
//   - Simple Fe sequences: ESC followed by a single byte in @-_
//     e.g. \x1b= \x1b>
//   - Any lone ESC byte not matched by the above
var nonColorEscapeRE = regexp.MustCompile(
	`\x1b\[[0-9;:<=>?]*[ -/]*[A-Za-z@-~]` + // CSI sequences
		`|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)` + // OSC sequences (terminated by BEL or ST)
		`|\x1b[@-_]` + // Fe two-byte sequences
		`|\x1b`, // any remaining lone ESC
)

// Run executes the configured extra command and returns its label output.
// Returns empty string on timeout, error, or empty command.
func Run(command string) string {
	if command == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return parseLabel(string(out))
}

// RunWidget executes a user-configured widget command with stdinJSON piped to
// its stdin and returns the sanitized first line of stdout. Unlike Run, the
// output contract is plain text rendered verbatim (SGR colors preserved), not
// a JSON label. Returns empty string on error, timeout, or empty command, so
// the widget disappears instead of blocking the line.
func RunWidget(command string, stdinJSON []byte, timeout time.Duration) string {
	if command == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = bytes.NewReader(stdinJSON)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	firstLine, _, _ := strings.Cut(string(out), "\n")
	return sanitize(firstLine)
}

// labelOutput is the expected JSON shape of the command's stdout.
type labelOutput struct {
	Label string `json:"label"`
}

// parseLabel parses the JSON output and returns the sanitized label string.
// Returns empty string if parsing fails or the label field is absent.
func parseLabel(raw string) string {
	var result labelOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &result); err != nil {
		return ""
	}
	return sanitize(result.Label)
}

// sanitize strips non-printable characters from s while preserving ANSI color
// escape sequences (\x1b[...m). This prevents terminal escape injection while
// allowing user-specified colors.
//
// Kept:    printable Unicode, ANSI SGR sequences (\x1b[ digits/semicolons m)
// Stripped: all other bytes < 0x20, DEL (0x7F), and non-color escape sequences
//
//	(cursor movement, OSC title sequences, etc.)
func sanitize(s string) string {
	// Step 1: protect ANSI color escape sequences with unique placeholders so
	// they survive the subsequent stripping passes.
	type placeholder struct {
		token string
		value string
	}

	var tokens []placeholder
	tokenIndex := 0

	// Replace each color escape with a sentinel containing only printable ASCII.
	result := ansiColorRE.ReplaceAllStringFunc(s, func(match string) string {
		key := "\x00ANSI_" + string(rune('A'+tokenIndex)) + "\x00"
		tokens = append(tokens, placeholder{token: key, value: match})
		tokenIndex++
		return key
	})

	// Step 2: strip all remaining non-color ESC sequences (cursor movement,
	// OSC title sequences, etc.) now that the color escapes are protected.
	result = nonColorEscapeRE.ReplaceAllString(result, "")

	// Step 3: strip remaining control characters (< 0x20) and DEL (0x7F).
	// The \x00 bytes that bracket our sentinels are control chars but we handle
	// them specially: keep them so we can restore the placeholders.
	var b strings.Builder
	b.Grow(len(result))
	for _, r := range result {
		if r == '\x00' {
			// Sentinel boundary character — keep for placeholder restoration.
			b.WriteRune(r)
			continue
		}
		if r == unicode.ReplacementChar {
			continue
		}
		if r < 0x20 || r == 0x7F {
			// Other control characters — strip.
			continue
		}
		b.WriteRune(r)
	}
	cleaned := b.String()

	// Step 4: restore the original ANSI color escape sequences.
	for _, p := range tokens {
		cleaned = strings.ReplaceAll(cleaned, p.token, p.value)
	}

	// Remove any residual \x00 sentinel bytes.
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")

	return strings.TrimSpace(cleaned)
}
