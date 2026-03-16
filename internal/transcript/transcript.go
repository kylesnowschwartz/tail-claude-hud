// Package transcript parses JSONL transcript files produced by Claude Code.
// It provides entry-level parsing and content block extraction for use by
// higher-level extraction layers that populate model.TranscriptData.
package transcript

import (
	"encoding/json"
	"fmt"
	"time"
)

// Entry represents a single JSONL line from a Claude Code session file.
// Fields map to the on-disk format at ~/.claude/projects/{project}/{session}.jsonl.
// Only the fields needed for statusline rendering are kept here — LeafUUID,
// Summary, and SourceToolUseID are omitted as they are not needed downstream.
type Entry struct {
	Type      string `json:"type"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Slug      string `json:"slug"`

	// IsSidechain is true when this entry originates from an agent subprocess
	// rather than the main conversation thread. Sidechain entries represent
	// internal activity of sub-agents and must be filtered out before processing
	// to avoid double-counting tool calls and agent launches.
	IsSidechain bool `json:"isSidechain"`

	// CustomTitle is populated when Type is "custom-title".
	CustomTitle string `json:"customTitle"`

	Message struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		Model      string          `json:"model"`
		StopReason *string         `json:"stop_reason"`
		Usage      *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParsedTimestamp returns the entry's Timestamp as a time.Time.
// Returns the zero value if the timestamp is missing or unparseable.
func (e Entry) ParsedTimestamp() time.Time {
	return parseTimestamp(e.Timestamp)
}

// ToolUseBlock represents a tool_use content block from an assistant message.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlock represents a tool_result content block from a user message.
type ToolResultBlock struct {
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// ThinkingBlock represents a thinking content block from an assistant message.
// The actual thinking text is intentionally omitted — only presence matters for
// the statusline.
type ThinkingBlock struct{}

// ContentBlocks holds the extracted content blocks from a single message.
// Blocks are classified during parsing; callers access only the types they need.
type ContentBlocks struct {
	ToolUse    []ToolUseBlock
	ToolResult []ToolResultBlock
	Thinking   []ThinkingBlock
	HasText    bool // true when at least one "text" block is present
}

// ParseEntry parses a single JSONL line into an Entry.
// Returns an error if the JSON is invalid.
// Entries without a UUID are accepted — some entry types (e.g. custom-title)
// may omit the UUID field legitimately.
func ParseEntry(line []byte) (Entry, error) {
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return Entry{}, fmt.Errorf("transcript: invalid JSON: %w", err)
	}
	return e, nil
}

// ExtractContentBlocks walks message.content and classifies each block by type.
// Unrecognised block types are silently ignored. Returns nil blocks (not an
// error) when content is absent or not a JSON array.
func ExtractContentBlocks(e Entry) ContentBlocks {
	if len(e.Message.Content) == 0 {
		return ContentBlocks{}
	}

	// Content can be a JSON string (plain text) or an array of typed blocks.
	// Only arrays contain tool_use / tool_result / thinking blocks.
	if e.Message.Content[0] != '[' {
		return ContentBlocks{}
	}

	// rawBlock lets us inspect only the "type" field before full unmarshalling,
	// avoiding allocations for blocks we won't use.
	type rawBlock struct {
		Type string `json:"type"`
	}
	var raw []rawBlock
	if err := json.Unmarshal(e.Message.Content, &raw); err != nil {
		return ContentBlocks{}
	}

	var result ContentBlocks
	for i, rb := range raw {
		switch rb.Type {
		case "tool_use":
			var block struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if err := unmarshalNthBlock(e.Message.Content, i, &block); err == nil {
				result.ToolUse = append(result.ToolUse, ToolUseBlock{
					ID:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				})
			}
		case "tool_result":
			var block struct {
				ToolUseID string          `json:"tool_use_id"`
				Content   json.RawMessage `json:"content"`
				IsError   bool            `json:"is_error"`
			}
			if err := unmarshalNthBlock(e.Message.Content, i, &block); err == nil {
				result.ToolResult = append(result.ToolResult, ToolResultBlock{
					ToolUseID: block.ToolUseID,
					Content:   block.Content,
					IsError:   block.IsError,
				})
			}
		case "thinking":
			result.Thinking = append(result.Thinking, ThinkingBlock{})
		case "text":
			result.HasText = true
		}
	}
	return result
}

// ParseTranscriptFile reads a JSONL transcript file and returns all successfully
// parsed entries. Lines that fail to parse are skipped. This is the intended
// top-level entry point for the --dump-current integration path in main.go.
func ParseTranscriptFile(data []byte) []Entry {
	var entries []Entry
	start := 0
	for i := 0; i <= len(data); i++ {
		if i == len(data) || data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			// Skip blank lines
			if len(line) == 0 {
				continue
			}
			e, err := ParseEntry(line)
			if err != nil {
				continue
			}
			entries = append(entries, e)
		}
	}
	return entries
}

// parseTimestamp parses an ISO 8601 timestamp string using multiple format
// variations. Claude Code emits inconsistent formats — some include nanoseconds,
// some have no timezone suffix. Returns the zero time on failure.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Most common: full RFC3339 with nanoseconds.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	// RFC3339 without sub-seconds.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// No timezone suffix — Claude sometimes emits this variant.
	if t, err := time.Parse("2006-01-02T15:04:05.999999999", s); err == nil {
		return t
	}
	return time.Time{}
}

// unmarshalNthBlock unmarshals the nth element of a JSON array into dest.
// Avoids re-parsing the whole array when only one element is needed.
func unmarshalNthBlock(raw json.RawMessage, n int, dest interface{}) error {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	if n >= len(items) {
		return fmt.Errorf("transcript: index %d out of range (len %d)", n, len(items))
	}
	return json.Unmarshal(items[n], dest)
}
