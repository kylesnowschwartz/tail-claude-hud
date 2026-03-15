package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func defaultCfg() *config.Config {
	return config.LoadHud()
}

func TestModelWidget_DisplaysNameInBrackets(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Opus", ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Model(ctx, cfg)
	if !strings.Contains(got, "Opus") {
		t.Errorf("expected output to contain 'Opus', got %q", got)
	}
	if !strings.Contains(got, "200k context") {
		t.Errorf("expected context size '200k context', got %q", got)
	}
}

func TestModelWidget_HidesContextSize(t *testing.T) {
	ctx := &model.RenderContext{ModelDisplayName: "Sonnet", ContextWindowSize: 200000}
	cfg := defaultCfg()
	cfg.Model.ShowContextSize = false

	got := Model(ctx, cfg)
	if strings.Contains(got, "context") {
		t.Errorf("expected no context size when disabled, got %q", got)
	}
	if !strings.Contains(got, "Sonnet") {
		t.Errorf("expected 'Sonnet' in output, got %q", got)
	}
}

func TestModelWidget_EmptyName(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Model(ctx, cfg); got != "" {
		t.Errorf("expected empty string for empty model name, got %q", got)
	}
}

func TestContextWidget_GreenUnder70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 50, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "50%") {
		t.Errorf("expected '50%%' in output, got %q", got)
	}
}

func TestContextWidget_YellowAt70(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 75, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "75%") {
		t.Errorf("expected '75%%' in output, got %q", got)
	}
}

func TestContextWidget_RedAt85(t *testing.T) {
	ctx := &model.RenderContext{ContextPercent: 90, ContextWindowSize: 200000}
	cfg := defaultCfg()

	got := Context(ctx, cfg)
	if !strings.Contains(got, "90%") {
		t.Errorf("expected '90%%' in output, got %q", got)
	}
}

func TestContextWidget_EmptyWhenZero(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Context(ctx, cfg); got != "" {
		t.Errorf("expected empty string for zero context, got %q", got)
	}
}

func TestDirectoryWidget_SingleSegment(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Levels = 1

	got := Directory(ctx, cfg)
	if !strings.Contains(got, "tail-claude-hud") {
		t.Errorf("expected 'tail-claude-hud', got %q", got)
	}
}

func TestDirectoryWidget_MultipleSegments(t *testing.T) {
	ctx := &model.RenderContext{Cwd: "/Users/kyle/Code/my-projects/tail-claude-hud"}
	cfg := defaultCfg()
	cfg.Directory.Levels = 2

	got := Directory(ctx, cfg)
	if !strings.Contains(got, "my-projects/tail-claude-hud") {
		t.Errorf("expected 2 segments, got %q", got)
	}
}

func TestDirectoryWidget_EmptyCwd(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	if got := Directory(ctx, cfg); got != "" {
		t.Errorf("expected empty string for empty cwd, got %q", got)
	}
}

func TestRegistryHasAllWidgets(t *testing.T) {
	expected := []string{"model", "context", "directory", "git", "env", "duration", "usage", "tools", "agents", "todos"}
	for _, name := range expected {
		if _, ok := Registry[name]; !ok {
			t.Errorf("Registry missing widget %q", name)
		}
	}
	if len(Registry) != len(expected) {
		t.Errorf("Registry has %d entries, expected %d", len(Registry), len(expected))
	}
}

func TestPlaceholderReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{}
	cfg := defaultCfg()

	// Phase 3 placeholders — still unimplemented.
	placeholders := []string{"tools", "agents", "todos"}
	for _, name := range placeholders {
		fn := Registry[name]
		if got := fn(ctx, cfg); got != "" {
			t.Errorf("placeholder widget %q returned %q, expected empty", name, got)
		}
	}
}

// -- Env widget ---------------------------------------------------------------

func TestEnvWidget_NilEnvCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: nil}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); got != "" {
		t.Errorf("Env with nil EnvCounts: expected empty, got %q", got)
	}
}

func TestEnvWidget_ShowsMCPCount(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 3, ToolsAllowed: 0}}
	cfg := defaultCfg()

	got := Env(ctx, cfg)
	if !strings.Contains(got, "3") {
		t.Errorf("Env: expected '3' in output, got %q", got)
	}
}

func TestEnvWidget_ShowsToolsAllowed(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 0, ToolsAllowed: 5}}
	cfg := defaultCfg()

	got := Env(ctx, cfg)
	if !strings.Contains(got, "5") {
		t.Errorf("Env: expected '5' in output, got %q", got)
	}
}

func TestEnvWidget_ZeroCountsReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 0, ToolsAllowed: 0}}
	cfg := defaultCfg()

	if got := Env(ctx, cfg); got != "" {
		t.Errorf("Env with all-zero counts: expected empty, got %q", got)
	}
}

func TestEnvWidget_UsesIconLookup(t *testing.T) {
	ctx := &model.RenderContext{EnvCounts: &model.EnvCounts{MCPServers: 1, ToolsAllowed: 0}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Env(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Spinner) {
		t.Errorf("Env(ascii): expected spinner icon %q, got %q", icons.Spinner, got)
	}
}

// -- Duration widget ----------------------------------------------------------

func TestDurationWidget_EmptySessionDurationReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{SessionDuration: ""}
	cfg := defaultCfg()

	if got := Duration(ctx, cfg); got != "" {
		t.Errorf("Duration with empty SessionDuration: expected empty, got %q", got)
	}
}

func TestDurationWidget_RendersTimestamp(t *testing.T) {
	// Use a timestamp 2 hours and 30 minutes ago.
	start := time.Now().Add(-2*time.Hour - 30*time.Minute).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionDuration: start}
	cfg := defaultCfg()

	got := Duration(ctx, cfg)
	if !strings.Contains(got, "h") {
		t.Errorf("Duration >= 1h: expected 'h' in output, got %q", got)
	}
	if !strings.Contains(got, "m") {
		t.Errorf("Duration >= 1h: expected 'm' in output, got %q", got)
	}
}

func TestDurationWidget_ShortSession(t *testing.T) {
	start := time.Now().Add(-5*time.Minute - 10*time.Second).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionDuration: start}
	cfg := defaultCfg()

	got := Duration(ctx, cfg)
	if !strings.Contains(got, "m") {
		t.Errorf("Duration < 1h: expected 'm' in output, got %q", got)
	}
	if !strings.Contains(got, "s") {
		t.Errorf("Duration < 1h: expected 's' in output, got %q", got)
	}
}

func TestDurationWidget_UsesIconLookup(t *testing.T) {
	start := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	ctx := &model.RenderContext{SessionDuration: start}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Duration(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Clock) {
		t.Errorf("Duration(ascii): expected clock icon %q, got %q", icons.Clock, got)
	}
}

// -- Git widget ---------------------------------------------------------------

func TestGitWidget_NilGitReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Git: nil}
	cfg := defaultCfg()

	if got := Git(ctx, cfg); got != "" {
		t.Errorf("Git with nil Git: expected empty, got %q", got)
	}
}

func TestGitWidget_ShowsBranch(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()

	got := Git(ctx, cfg)
	if !strings.Contains(got, "main") {
		t.Errorf("Git: expected 'main' in output, got %q", got)
	}
}

func TestGitWidget_DirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/foo", Dirty: true}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg)
	if !strings.Contains(got, "*") {
		t.Errorf("Git dirty: expected '*' in output, got %q", got)
	}
}

func TestGitWidget_CleanNoDirtyIndicator(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main", Dirty: false}}
	cfg := defaultCfg()
	cfg.Git.Dirty = true

	got := Git(ctx, cfg)
	if strings.Contains(got, "*") {
		t.Errorf("Git clean: expected no '*', got %q", got)
	}
}

func TestGitWidget_AheadBehindCounts(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "feat/bar", AheadBy: 2, BehindBy: 1}}
	cfg := defaultCfg()
	cfg.Git.AheadBehind = true

	got := Git(ctx, cfg)
	if !strings.Contains(got, "+2") {
		t.Errorf("Git ahead: expected '+2', got %q", got)
	}
	if !strings.Contains(got, "-1") {
		t.Errorf("Git behind: expected '-1', got %q", got)
	}
}

func TestGitWidget_UsesIconLookup(t *testing.T) {
	ctx := &model.RenderContext{Git: &model.GitStatus{Branch: "main"}}
	cfg := defaultCfg()
	cfg.Style.Icons = "ascii"

	got := Git(ctx, cfg)
	icons := IconsFor("ascii")
	if !strings.Contains(got, icons.Branch) {
		t.Errorf("Git(ascii): expected branch icon %q, got %q", icons.Branch, got)
	}
}

// -- Usage widget -------------------------------------------------------------

func TestUsageWidget_NilUsageReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Usage: nil}
	cfg := defaultCfg()

	if got := Usage(ctx, cfg); got != "" {
		t.Errorf("Usage with nil Usage: expected empty, got %q", got)
	}
}

func TestUsageWidget_ZeroDataReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{Usage: &model.UsageData{ContextPercent: 0, ContextWindowSize: 0}}
	cfg := defaultCfg()

	if got := Usage(ctx, cfg); got != "" {
		t.Errorf("Usage with zero data: expected empty, got %q", got)
	}
}

func TestUsageWidget_ShowsPercent(t *testing.T) {
	ctx := &model.RenderContext{Usage: &model.UsageData{ContextPercent: 42, ContextWindowSize: 200000}}
	cfg := defaultCfg()

	got := Usage(ctx, cfg)
	if !strings.Contains(got, "42%") {
		t.Errorf("Usage: expected '42%%' in output, got %q", got)
	}
}

func TestUsageWidget_BarContainsBlocks(t *testing.T) {
	ctx := &model.RenderContext{Usage: &model.UsageData{ContextPercent: 50, ContextWindowSize: 200000}}
	cfg := defaultCfg()

	got := Usage(ctx, cfg)
	if !strings.Contains(got, "█") {
		t.Errorf("Usage bar: expected filled blocks in output, got %q", got)
	}
	if !strings.Contains(got, "░") {
		t.Errorf("Usage bar: expected empty blocks in output, got %q", got)
	}
}

func TestIconsFor_Modes(t *testing.T) {
	tests := []struct {
		mode      string
		wantCheck string
	}{
		{"unicode", "✓"},
		{"ascii", "v"},
	}
	for _, tt := range tests {
		icons := IconsFor(tt.mode)
		if icons.Check != tt.wantCheck {
			t.Errorf("IconsFor(%q).Check = %q, want %q", tt.mode, icons.Check, tt.wantCheck)
		}
	}

	// Nerdfont should return non-empty
	nf := IconsFor("nerdfont")
	if nf.Check == "" {
		t.Error("nerdfont Check icon is empty")
	}

	// Unknown mode falls back to ascii
	unk := IconsFor("unknown")
	if unk.Check != "v" {
		t.Errorf("unknown mode should fall back to ascii, got Check=%q", unk.Check)
	}
}

func TestLastNSegments(t *testing.T) {
	tests := []struct {
		path string
		n    int
		want string
	}{
		{"/Users/kyle/Code", 1, "Code"},
		{"/Users/kyle/Code", 2, "kyle/Code"},
		{"/Users/kyle/Code", 5, "Users/kyle/Code"},
		{"relative/path", 1, "path"},
		{"/trailing/slash/", 1, "slash"},
		{"", 1, ""},
		{"/", 1, ""},
	}

	for _, tt := range tests {
		got := lastNSegments(tt.path, tt.n)
		if got != tt.want {
			t.Errorf("lastNSegments(%q, %d) = %q, want %q", tt.path, tt.n, got, tt.want)
		}
	}
}
