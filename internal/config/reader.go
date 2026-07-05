// Package config loads TOML-based HUD configuration with defaults.
// LoadHud never returns nil and never returns an error — it fails open,
// using defaults whenever the config file is absent or unreadable.
package config

import (
	"os"
	"path/filepath"

	"github.com/kylesnowschwartz/agent-ouija/claude/settings"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// CountEnv counts the active Claude Code environment config items for the given
// working directory and returns an EnvCounts. It never returns nil.
//
// MCPServers counts unique server names across:
//   - ~/.claude/settings.json
//   - {cwd}/.claude/settings.json
//   - {cwd}/.claude/settings.local.json
//   - {cwd}/.mcp.json
//
// ClaudeMdFiles counts CLAUDE.md files that exist at standard paths.
//
// RuleFiles counts .md files under ~/.claude/rules/ and {cwd}/.claude/rules/.
//
// Hooks counts non-empty hook event arrays in all settings.json files.
//
// Missing files are silently skipped. Invalid JSON is silently skipped.
func CountEnv(cwd string) *model.EnvCounts {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return countEnvWithHome(cwd, home)
}

// countEnvWithHome is the testable core of CountEnv. Accepting home as a
// parameter lets tests substitute a temp directory instead of the real home.
func countEnvWithHome(cwd, home string) *model.EnvCounts {
	counts := &model.EnvCounts{}

	// --- MCPServers ---
	// Collect unique server names across all settings files. Per the card spec,
	// we count unique names globally (a name appearing in two files counts once).
	mcpNames := make(map[string]struct{})

	if home != "" {
		addMcpNames(filepath.Join(home, ".claude", "settings.json"), mcpNames)
	}
	if cwd != "" {
		addMcpNames(filepath.Join(cwd, ".claude", "settings.json"), mcpNames)
		addMcpNames(filepath.Join(cwd, ".claude", "settings.local.json"), mcpNames)
		addMcpNames(filepath.Join(cwd, ".mcp.json"), mcpNames)
	}
	counts.MCPServers = len(mcpNames)

	// --- ClaudeMdFiles ---
	for _, p := range claudeMdPaths(home, cwd) {
		if fileExists(p) {
			counts.ClaudeMdFiles++
		}
	}

	// --- RuleFiles: recursive .md count under rules directories ---
	if home != "" {
		counts.RuleFiles += countMdFilesRecursive(filepath.Join(home, ".claude", "rules"))
	}
	if cwd != "" {
		counts.RuleFiles += countMdFilesRecursive(filepath.Join(cwd, ".claude", "rules"))
	}

	// --- Hooks: count non-empty hook event arrays across settings files ---
	if home != "" {
		counts.Hooks += settings.NonEmptyHookCount(filepath.Join(home, ".claude", "settings.json"))
	}
	if cwd != "" {
		counts.Hooks += settings.NonEmptyHookCount(filepath.Join(cwd, ".claude", "settings.json"))
		counts.Hooks += settings.NonEmptyHookCount(filepath.Join(cwd, ".claude", "settings.local.json"))
	}

	return counts
}

// claudeMdPaths returns the candidate CLAUDE.md paths for the given home and cwd.
// Paths are returned in discovery order; callers check existence themselves.
func claudeMdPaths(home, cwd string) []string {
	var paths []string
	if home != "" {
		paths = append(paths, filepath.Join(home, ".claude", "CLAUDE.md"))
	}
	if cwd != "" {
		paths = append(paths,
			filepath.Join(cwd, "CLAUDE.md"),
			filepath.Join(cwd, "CLAUDE.local.md"),
			filepath.Join(cwd, ".claude", "CLAUDE.md"),
		)
	}
	return paths
}

// addMcpNames adds the mcpServers key names from a settings.json or
// .mcp.json file to the provided set. Missing or invalid files are skipped.
func addMcpNames(path string, names map[string]struct{}) {
	for _, name := range settings.McpServerNames(path) {
		names[name] = struct{}{}
	}
}

// countMdFilesRecursive returns the number of .md files under dir, recursing
// into subdirectories. Returns 0 if dir does not exist.
func countMdFilesRecursive(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count += countMdFilesRecursive(filepath.Join(dir, e.Name()))
		} else if filepath.Ext(e.Name()) == ".md" {
			count++
		}
	}
	return count
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
