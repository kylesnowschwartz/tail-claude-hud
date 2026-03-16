// Package model defines the shared data types passed between gather and render stages.
package model

import "time"

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

	// Pointer fields — all may be nil when the corresponding data is unavailable.
	Transcript *TranscriptData
	EnvCounts  *EnvCounts
	Git        *GitStatus
}

// TranscriptData holds parsed information from the Claude Code transcript.
type TranscriptData struct {
	Path        string
	SessionName string
	Tools       []ToolEntry
	Agents      []AgentEntry
	Todos       []TodoItem

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
}

// ToolEntry records a single tool invocation observed in the transcript.
type ToolEntry struct {
	Name       string
	Count      int
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

	// ContextPercent is computed by the stdin package — not decoded from JSON.
	ContextPercent int `json:"-"`
}
