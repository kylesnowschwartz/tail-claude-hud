// Package model defines the shared data types passed between gather and render stages.
package model

import (
	"os"
	"path/filepath"
	"time"
)

// RenderContext is the central struct passed from the gather stage to each render widget.
// Every pointer field may be nil — widgets must guard against nil before dereferencing.
type RenderContext struct {
	TerminalWidth int
	SessionStart  string

	// Top-level fields populated from StdinData during the gather stage.
	// Widgets read these directly rather than dereferencing pointer fields.
	ModelID           string
	ModelDisplayName  string
	ContextWindowSize int
	ContextPercent    int
	Cwd               string

	// Token fields populated from StdinData.ContextWindow.CurrentUsage.
	InputTokens   int
	CacheCreation int
	CacheRead     int

	// Cost and duration fields from StdinData.Cost.
	// SessionCostUSD is 0 when no cost data is available.
	// TotalDurationMs is the authoritative session duration from Claude Code;
	// prefer it over transcript-derived duration when non-zero.
	SessionCostUSD  float64
	TotalDurationMs int
	ApiDurationMs   int
	LinesAdded      int
	LinesRemoved    int

	// OutputStyle is the current output style name (e.g. "auto", "verbose").
	// Empty string when not provided by Claude Code.
	OutputStyle string


	// Pointer fields — all may be nil when the corresponding data is unavailable.
	Transcript *TranscriptData
	EnvCounts  *EnvCounts
	Git        *GitStatus
}

// TokenSample records a token count observation at a point in time.
// It is used by the speed widget to compute a rolling tokens/sec average.
type TokenSample struct {
	Timestamp time.Time
	Tokens    int // total tokens (input + output) from a single assistant message
}

// TranscriptData holds parsed information from the Claude Code transcript.
type TranscriptData struct {
	Path        string
	SessionName string
	Tools       []ToolEntry
	Agents      []AgentEntry
	Todos       []TodoItem
	// SkillNames is the ordered list of skill names invoked in the session
	// (newest last), capped at 20. Each entry is the full skill identifier
	// extracted from a "Skill" tool_use block's input.skill field.
	SkillNames []string

	// TokenSamples holds timestamp+token pairs extracted from assistant messages.
	// Used by the speed widget to compute a rolling tokens/sec average.
	TokenSamples []TokenSample

	// ThinkingActive is true when the most recent assistant message contained a
	// thinking block that was not followed by a tool_use or text block in the
	// same message.
	ThinkingActive bool

	// ThinkingCount is the total number of thinking blocks observed across all
	// assistant messages in the session.
	ThinkingCount int

	// SpinnerFrame is a monotonic counter incremented on each statusline invocation.
	// Widgets use it instead of wall-clock time to guarantee spinner advancement
	// on every render regardless of when within a tick the binary runs.
	SpinnerFrame int

	// DividerOffset is a monotonic counter incremented once per new tool_use.
	// The tools widget uses it to highlight one separator: position = offset %
	// (numVisible - 1). This creates a scrolling ticker effect where the
	// highlighted divider advances with each new tool call and wraps around.
	DividerOffset int

	// MessageCount is the number of user/assistant conversational turns in the
	// transcript, excluding pure tool_result entries.
	MessageCount int
}

// ToolEntry records a single tool invocation observed in the transcript.
type ToolEntry struct {
	Name       string
	Completed  bool   // false = still running, true = completed or error
	DurationMs int    // 0 = still running or unknown
	HasError   bool   // true when the tool_result had is_error set
	Category   string // file, shell, search, web, agent, internal
	Target     string // file path, command, pattern, or other contextual string
}

// AgentEntry records a sub-agent task observed in the transcript.
type AgentEntry struct {
	Name        string
	Status      string
	Model       string    // e.g. "claude-haiku-4-5"
	Description string    // agent task description from the tool input
	ColorIndex  int       // 0-7, assigned by first-appearance order (index % 8)
	StartTime   time.Time // when the agent tool_use was observed
	DurationMs  int       // 0 = still running; populated by a separate card
}

// TodoItem represents a todo entry from the Claude Code session.
type TodoItem struct {
	ID      string
	Content string
	Done    bool
}

// EnvCounts holds counts of active Claude Code environment config items,
// broken down by category.
type EnvCounts struct {
	MCPServers    int // unique MCP server names across all settings files
	ClaudeMdFiles int // CLAUDE.md files found at standard paths
	RuleFiles     int // .md files under ~/.claude/rules and {cwd}/.claude/rules
	Hooks         int // non-empty hook event arrays across settings files
}

// GitStatus holds the current git repository state for the working directory.
type GitStatus struct {
	Branch    string
	Dirty     bool
	AheadBy   int
	BehindBy  int
	Untracked int
	Modified  int
	Staged    int
}

// IsDirty reports whether the working tree has any uncommitted changes
// (dirty flag, modified files, staged files, or untracked files).
func (g *GitStatus) IsDirty() bool {
	return g.Dirty || g.Modified > 0 || g.Staged > 0 || g.Untracked > 0
}

// Cost holds session-level cost and duration metrics from Claude Code's stdin JSON.
// All fields are optional; zero values indicate the data was not provided.
type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMs    int     `json:"total_duration_ms"`
	TotalAPIDurationMs int     `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

// OutputStyle holds the current output style configuration from Claude Code.
type OutputStyle struct {
	Name string `json:"name"`
}

// StdinData is the raw decoded form of the JSON blob Claude Code pipes to stdin.
// It is produced by the stdin package and then transformed into RenderContext fields.
type StdinData struct {
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Model          *struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow *struct {
		Size         int      `json:"context_window_size"`
		UsedPercent  *float64 `json:"used_percentage"`
		CurrentUsage *struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`

	// Cost is nil when Claude Code does not include cost data in the stdin payload.
	Cost *Cost `json:"cost"`

	// OutputStyle is nil when Claude Code does not include output_style in the stdin payload.
	OutputStyle *OutputStyle `json:"output_style"`


	// ContextPercent is computed by the stdin package — not decoded from JSON.
	ContextPercent int `json:"-"`
}

// PluginDir returns the directory used for plugin state files:
// ~/.claude/plugins/tail-claude-hud/
// Falls back to os.TempDir() if the home directory cannot be resolved.
func PluginDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".claude", "plugins", "tail-claude-hud")
}
