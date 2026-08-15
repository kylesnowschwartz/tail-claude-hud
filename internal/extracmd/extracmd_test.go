package extracmd

import (
	"strings"
	"testing"
	"time"
)

// --- Run ---

func TestRun_EmptyCommand(t *testing.T) {
	// Spec 1: no process should be spawned, empty string returned.
	got := Run("")
	if got != "" {
		t.Errorf("Run(\"\") = %q, want \"\"", got)
	}
}

func TestRun_ValidJSONLabel(t *testing.T) {
	// Spec 3: valid JSON with a label field returns the sanitized label.
	got := Run(`echo '{"label":"hello"}'`)
	if got != "hello" {
		t.Errorf("Run(echo hello) = %q, want \"hello\"", got)
	}
}

func TestRun_CommandFails(t *testing.T) {
	got := Run("exit 1")
	if got != "" {
		t.Errorf("Run(exit 1) = %q, want \"\"", got)
	}
}

func TestRun_InvalidJSON(t *testing.T) {
	got := Run("echo 'not json'")
	if got != "" {
		t.Errorf("Run(echo not json) = %q, want \"\"", got)
	}
}

func TestRun_MissingLabelField(t *testing.T) {
	got := Run(`echo '{"other":"value"}'`)
	if got != "" {
		t.Errorf("Run missing label field = %q, want \"\"", got)
	}
}

func TestRun_Timeout(t *testing.T) {
	// Spec 2: commands that exceed 3 seconds must return "".
	// Use sleep 10 — well beyond the 3s timeout.
	got := Run("sleep 10")
	if got != "" {
		t.Errorf("Run(sleep 10) = %q, want \"\" (should have timed out)", got)
	}
}

func TestRun_ANSIColorInLabel(t *testing.T) {
	// Spec 3 + 4: ANSI color codes in the label are preserved after sanitization.
	// JSON \u001b decodes to the ESC byte, so the command below produces valid
	// JSON with actual ESC bytes that our sanitizer must preserve.
	got := Run(`echo '{"label":"\u001b[31mred\u001b[0m"}'`)
	if !strings.Contains(got, "red") {
		t.Errorf("Run ANSI label = %q, want it to contain 'red'", got)
	}
	// The ESC byte should also be present (color code preserved).
	if !strings.Contains(got, "\x1b[31m") {
		t.Errorf("Run ANSI label = %q, want ANSI color escape preserved", got)
	}
}

// --- RunWidget ---

func TestRunWidget_EmptyCommand(t *testing.T) {
	if got := RunWidget("", nil, time.Second); got != "" {
		t.Errorf("RunWidget(\"\") = %q, want \"\"", got)
	}
}

func TestRunWidget_ReceivesStdin(t *testing.T) {
	got := RunWidget("cat", []byte(`{"session_id":"abc"}`), time.Second)
	if got != `{"session_id":"abc"}` {
		t.Errorf("RunWidget(cat) = %q, want stdin echoed back", got)
	}
}

func TestRunWidget_FirstLineOnly(t *testing.T) {
	got := RunWidget(`printf 'line1\nline2\n'`, nil, time.Second)
	if got != "line1" {
		t.Errorf("RunWidget multiline = %q, want \"line1\"", got)
	}
}

func TestRunWidget_CommandFails(t *testing.T) {
	if got := RunWidget("exit 1", nil, time.Second); got != "" {
		t.Errorf("RunWidget(exit 1) = %q, want \"\"", got)
	}
}

func TestRunWidget_Timeout(t *testing.T) {
	if got := RunWidget("sleep 10", nil, 50*time.Millisecond); got != "" {
		t.Errorf("RunWidget(sleep 10) = %q, want \"\" (should have timed out)", got)
	}
}

func TestRunWidget_PreservesANSIColors(t *testing.T) {
	got := RunWidget(`printf '\033[32m50%%\033[0m'`, nil, time.Second)
	if !strings.Contains(got, "\x1b[32m") || !strings.Contains(got, "50%") {
		t.Errorf("RunWidget ANSI = %q, want SGR escapes and content preserved", got)
	}
}

// --- sanitize ---

func TestSanitize_PlainString(t *testing.T) {
	// Spec 4: printable characters pass through unchanged.
	got := sanitize("hello world")
	if got != "hello world" {
		t.Errorf("sanitize plain = %q, want \"hello world\"", got)
	}
}

func TestSanitize_StripControlChars(t *testing.T) {
	// Spec 4: control characters (< 0x20) are stripped.
	got := sanitize("hel\x01lo\x08world")
	if got != "helloworld" {
		t.Errorf("sanitize control = %q, want \"helloworld\"", got)
	}
}

func TestSanitize_StripDEL(t *testing.T) {
	// Spec 4: DEL (0x7F) is stripped.
	got := sanitize("hel\x7flo")
	if got != "hello" {
		t.Errorf("sanitize DEL = %q, want \"hello\"", got)
	}
}

func TestSanitize_PreserveANSIColors(t *testing.T) {
	// Spec 4: ANSI SGR sequences are preserved.
	input := "\x1b[31mred\x1b[0m"
	got := sanitize(input)
	if got != input {
		t.Errorf("sanitize ANSI = %q, want %q", got, input)
	}
}

func TestSanitize_PreserveBoldColor(t *testing.T) {
	input := "\x1b[1;32mbold green\x1b[0m"
	got := sanitize(input)
	if got != input {
		t.Errorf("sanitize bold color = %q, want %q", got, input)
	}
}

func TestSanitize_StripCursorMovement(t *testing.T) {
	// Spec 4: non-color CSI sequences (e.g. cursor movement \x1b[2J) are stripped.
	// \x1b[2J clears the screen — not an SGR sequence (ends with J, not m).
	input := "\x1b[2Jhello"
	got := sanitize(input)
	if got != "hello" {
		t.Errorf("sanitize cursor movement = %q, want \"hello\"", got)
	}
}

func TestSanitize_StripOtherEscapes(t *testing.T) {
	// Non-color escape (e.g. \x1b] OSC) is stripped.
	// \x1b]0;title\x07 sets the terminal title.
	input := "\x1b]0;title\x07hello"
	got := sanitize(input)
	if got != "hello" {
		t.Errorf("sanitize OSC = %q, want \"hello\"", got)
	}
}

func TestSanitize_MixedContent(t *testing.T) {
	// Spec 4: real-world mix — color codes interleaved with control chars.
	// \x01 (SOH) should be stripped; color codes kept.
	input := "\x1b[32m\x01green\x1b[0m"
	got := sanitize(input)
	want := "\x1b[32mgreen\x1b[0m"
	if got != want {
		t.Errorf("sanitize mixed = %q, want %q", got, want)
	}
}

func TestSanitize_EmptyString(t *testing.T) {
	got := sanitize("")
	if got != "" {
		t.Errorf("sanitize empty = %q, want \"\"", got)
	}
}

func TestSanitize_TrimSpace(t *testing.T) {
	got := sanitize("  hello  ")
	if got != "hello" {
		t.Errorf("sanitize trim = %q, want \"hello\"", got)
	}
}
