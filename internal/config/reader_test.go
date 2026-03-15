package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes content to path, creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// TestCountEnv_EmptyCwd verifies that an empty cwd and empty home produces zero
// counts without errors.
func TestCountEnv_EmptyCwd(t *testing.T) {
	home := t.TempDir()
	counts := countEnvWithHome("", home)
	if counts == nil {
		t.Fatal("countEnvWithHome returned nil")
	}
	if counts.MCPServers != 0 {
		t.Errorf("MCPServers = %d, want 0", counts.MCPServers)
	}
	if counts.ToolsAllowed != 0 {
		t.Errorf("ToolsAllowed = %d, want 0", counts.ToolsAllowed)
	}
}

// TestCountEnv_ClaudeMdFiles verifies CLAUDE.md files are counted from the
// standard cwd locations.
func TestCountEnv_ClaudeMdFiles(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# project instructions")
	writeFile(t, filepath.Join(dir, "CLAUDE.local.md"), "# local overrides")
	writeFile(t, filepath.Join(dir, ".claude", "CLAUDE.md"), "# claude dir")

	counts := countEnvWithHome(dir, home)
	if counts.ToolsAllowed != 3 {
		t.Errorf("ToolsAllowed = %d, want 3 (3 CLAUDE.md files)", counts.ToolsAllowed)
	}
}

// TestCountEnv_HomeScopeClaudeMd verifies the home-scope CLAUDE.md is counted.
func TestCountEnv_HomeScopeClaudeMd(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# user instructions")

	counts := countEnvWithHome(dir, home)
	if counts.ToolsAllowed != 1 {
		t.Errorf("ToolsAllowed = %d, want 1 (home CLAUDE.md)", counts.ToolsAllowed)
	}
}

// TestCountEnv_RulesFiles verifies .md files in rules directories are counted.
func TestCountEnv_RulesFiles(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "rules", "coding.md"), "# coding rules")
	writeFile(t, filepath.Join(dir, ".claude", "rules", "writing.md"), "# writing rules")
	writeFile(t, filepath.Join(dir, ".claude", "rules", "nested", "security.md"), "# security")

	counts := countEnvWithHome(dir, home)
	if counts.ToolsAllowed != 3 {
		t.Errorf("ToolsAllowed = %d, want 3 (3 rule files)", counts.ToolsAllowed)
	}
}

// TestCountEnv_RulesIgnoresNonMd verifies non-.md files in rules dirs are skipped.
func TestCountEnv_RulesIgnoresNonMd(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "rules", "coding.md"), "# rule")
	writeFile(t, filepath.Join(dir, ".claude", "rules", "notes.txt"), "ignored")
	writeFile(t, filepath.Join(dir, ".claude", "rules", "config.json"), "{}")

	counts := countEnvWithHome(dir, home)
	if counts.ToolsAllowed != 1 {
		t.Errorf("ToolsAllowed = %d, want 1 (only .md files count)", counts.ToolsAllowed)
	}
}

// TestCountEnv_McpServersFromSettingsJson verifies MCP servers are counted from
// settings.json files.
func TestCountEnv_McpServersFromSettingsJson(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{
		"mcpServers": {
			"filesystem": {},
			"github": {}
		}
	}`)

	counts := countEnvWithHome(dir, home)
	if counts.MCPServers != 2 {
		t.Errorf("MCPServers = %d, want 2", counts.MCPServers)
	}
}

// TestCountEnv_McpServersFromMcpJson verifies MCP servers are counted from .mcp.json.
func TestCountEnv_McpServersFromMcpJson(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".mcp.json"), `{
		"mcpServers": {
			"local-tool": {},
			"dev-tool": {}
		}
	}`)

	counts := countEnvWithHome(dir, home)
	if counts.MCPServers != 2 {
		t.Errorf("MCPServers = %d, want 2", counts.MCPServers)
	}
}

// TestCountEnv_McpServersMergedAcrossFiles verifies that server names are
// deduplicated across settings files (same name in two files counts once).
func TestCountEnv_McpServersMergedAcrossFiles(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{
		"mcpServers": { "shared": {}, "project-only": {} }
	}`)
	writeFile(t, filepath.Join(dir, ".claude", "settings.local.json"), `{
		"mcpServers": { "shared": {}, "local-only": {} }
	}`)

	counts := countEnvWithHome(dir, home)
	// "shared" deduped, "project-only" and "local-only" each count once → 3 total
	if counts.MCPServers != 3 {
		t.Errorf("MCPServers = %d, want 3 (deduplicated across files)", counts.MCPServers)
	}
}

// TestCountEnv_McpServersDeduplicatedWithHome verifies home and cwd server names
// are also deduplicated against each other.
func TestCountEnv_McpServersDeduplicatedWithHome(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{
		"mcpServers": { "shared": {}, "home-only": {} }
	}`)
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{
		"mcpServers": { "shared": {}, "project-only": {} }
	}`)

	counts := countEnvWithHome(dir, home)
	// shared (deduped), home-only, project-only = 3
	if counts.MCPServers != 3 {
		t.Errorf("MCPServers = %d, want 3", counts.MCPServers)
	}
}

// TestCountEnv_HooksCountedAsToolsAllowed verifies that non-empty hooks arrays
// in settings.json contribute to ToolsAllowed.
func TestCountEnv_HooksCountedAsToolsAllowed(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "lint"}]}],
			"PostToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "notify"}]}],
			"Stop": []
		}
	}`)

	counts := countEnvWithHome(dir, home)
	// 2 non-empty hook arrays (Stop is empty so excluded)
	if counts.ToolsAllowed != 2 {
		t.Errorf("ToolsAllowed = %d, want 2 (2 non-empty hook arrays)", counts.ToolsAllowed)
	}
}

// TestCountEnv_MissingFilesSkippedWithoutError verifies that a cwd with no
// config files at all produces zero counts and does not panic.
func TestCountEnv_MissingFilesSkippedWithoutError(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()
	// dir and home exist but have no files inside

	counts := countEnvWithHome(dir, home)
	if counts == nil {
		t.Fatal("countEnvWithHome returned nil for empty dir")
	}
	if counts.MCPServers != 0 {
		t.Errorf("MCPServers = %d, want 0", counts.MCPServers)
	}
	if counts.ToolsAllowed != 0 {
		t.Errorf("ToolsAllowed = %d, want 0", counts.ToolsAllowed)
	}
}

// TestCountEnv_InvalidJsonSkipped verifies that an invalid JSON file is skipped
// without returning an error and without contributing to counts.
func TestCountEnv_InvalidJsonSkipped(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{ this is not valid json`)
	writeFile(t, filepath.Join(dir, ".mcp.json"), `not json at all`)

	counts := countEnvWithHome(dir, home)
	if counts.MCPServers != 0 {
		t.Errorf("MCPServers = %d, want 0 (invalid JSON skipped)", counts.MCPServers)
	}
}

// TestCountEnv_AllSourcesCombined verifies that all sources contribute correctly
// to the final counts when used together.
func TestCountEnv_AllSourcesCombined(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()

	// Home CLAUDE.md: 1
	writeFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# user")

	// Home rules: 1
	writeFile(t, filepath.Join(home, ".claude", "rules", "global.md"), "rule")

	// cwd CLAUDE.md files: 2
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# project")
	writeFile(t, filepath.Join(dir, "CLAUDE.local.md"), "# local")

	// cwd rule files: 2
	writeFile(t, filepath.Join(dir, ".claude", "rules", "a.md"), "rule a")
	writeFile(t, filepath.Join(dir, ".claude", "rules", "b.md"), "rule b")

	// Hooks: 1 non-empty hook array
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{
		"mcpServers": { "alpha": {}, "beta": {} },
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": []}]
		}
	}`)

	// MCP from .mcp.json: 1 new, 1 duplicate of alpha
	writeFile(t, filepath.Join(dir, ".mcp.json"), `{
		"mcpServers": { "alpha": {}, "gamma": {} }
	}`)

	counts := countEnvWithHome(dir, home)

	// MCPServers: alpha (deduped), beta, gamma = 3
	if counts.MCPServers != 3 {
		t.Errorf("MCPServers = %d, want 3", counts.MCPServers)
	}

	// ToolsAllowed: 1 home CLAUDE.md + 1 home rule + 2 cwd CLAUDE.md + 2 cwd rules + 1 hook = 7
	if counts.ToolsAllowed != 7 {
		t.Errorf("ToolsAllowed = %d, want 7", counts.ToolsAllowed)
	}
}

// TestCountMdFilesRecursive verifies that countMdFilesRecursive handles nested
// directories correctly.
func TestCountMdFilesRecursive(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "a.md"), "")
	writeFile(t, filepath.Join(dir, "b.md"), "")
	writeFile(t, filepath.Join(dir, "sub", "c.md"), "")
	writeFile(t, filepath.Join(dir, "sub", "deep", "d.md"), "")
	writeFile(t, filepath.Join(dir, "ignored.txt"), "")

	got := countMdFilesRecursive(dir)
	if got != 4 {
		t.Errorf("countMdFilesRecursive = %d, want 4", got)
	}
}

// TestCountMdFilesRecursive_NonExistentDir verifies that a missing directory
// returns 0 without error.
func TestCountMdFilesRecursive_NonExistentDir(t *testing.T) {
	got := countMdFilesRecursive("/does/not/exist")
	if got != 0 {
		t.Errorf("countMdFilesRecursive = %d, want 0 for missing dir", got)
	}
}
