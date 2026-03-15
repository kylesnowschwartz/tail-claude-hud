// Package model defines the shared data types passed between gather and render stages.
package model

// RenderContext is the central struct passed from the gather stage to each render widget.
// Every pointer field may be nil — widgets must guard against nil before dereferencing.
type RenderContext struct {
	// Config will be *config.Config once that package exists.
	Config interface{}

	TerminalWidth   int
	SessionDuration string
	ExtraLabel      string

	// Pointer fields — all may be nil when the corresponding data is unavailable.
	Transcript *TranscriptData
	EnvCounts  *EnvCounts
	Git        *GitStatus
	Usage      *UsageData
}

// TranscriptData holds parsed information from the Claude Code transcript.
type TranscriptData struct {
	Path   string
	Tools  []ToolEntry
	Agents []AgentEntry
	Todos  []TodoItem
}

// ToolEntry records a single tool invocation observed in the transcript.
type ToolEntry struct {
	Name  string
	Count int
}

// AgentEntry records a sub-agent task observed in the transcript.
type AgentEntry struct {
	Name   string
	Status string
}

// TodoItem represents a todo entry from the Claude Code session.
type TodoItem struct {
	ID      string
	Content string
	Done    bool
}

// EnvCounts holds counts of active MCP servers and permitted tools.
type EnvCounts struct {
	MCPServers   int
	ToolsAllowed int
}

// GitStatus holds the current git repository state for the working directory.
type GitStatus struct {
	Branch     string
	Dirty      bool
	AheadBy    int
	BehindBy   int
	Untracked  int
	Modified   int
	Staged     int
}

// UsageData holds token usage and context window information for the current session.
type UsageData struct {
	ContextWindowSize int
	ContextPercent    int
	InputTokens       int
	CacheCreation     int
	CacheRead         int
	ModelID           string
	ModelDisplayName  string
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
		Size        int      `json:"context_window_size"`
		UsedPercent *float64 `json:"used_percentage"`
		CurrentUsage *struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`

	// ContextPercent is computed by the stdin package — not decoded from JSON.
	ContextPercent int `json:"-"`
}
