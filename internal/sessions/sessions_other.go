//go:build !darwin

// Package sessions provides a no-op stub on non-macOS platforms.
// The sysctl-based process detection is macOS-specific.
package sessions

// FindWaitingSession always returns nil on non-macOS platforms.
func FindWaitingSession(ownTranscript string) *WaitingSession {
	return nil
}

// WaitingSession holds information about another Claude Code session that is
// blocked waiting for user permission approval.
type WaitingSession struct {
	CWD     string
	Project string
}
