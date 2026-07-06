package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/git"
)

// initRepo creates a minimal git repo in dir with a configured identity so
// commits succeed in CI environments where global git config may be absent.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cmd := exec.Command("git", "add", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "add "+name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// TestGetStatus_NotARepo verifies GetStatus returns nil outside a git repo.
func TestGetStatus_NotARepo(t *testing.T) {
	dir := t.TempDir()
	status := git.GetStatus(dir)
	if status != nil {
		t.Fatalf("expected nil for non-git directory, got %+v", status)
	}
}

// TestGetStatus_CleanRepo verifies a clean repo with one commit reports the
// correct branch name and zero counts.
func TestGetStatus_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status for git repo")
	}
	if status.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	if status.Dirty {
		t.Error("expected clean repo to not be dirty")
	}
	if status.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", status.Modified)
	}
	if status.Staged != 0 {
		t.Errorf("expected 0 staged, got %d", status.Staged)
	}
	if status.Untracked != 0 {
		t.Errorf("expected 0 untracked, got %d", status.Untracked)
	}
}

// TestGetStatus_BranchName verifies the branch name is reported correctly after
// checking out a non-default branch.
func TestGetStatus_BranchName(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "file.txt", "content")

	cmd := exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b: %v\n%s", err, out)
	}

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Branch != "test-branch" {
		t.Errorf("expected branch 'test-branch', got %q", status.Branch)
	}
}

// TestGetStatus_UntrackedFile verifies untracked files are counted and Dirty is set.
func TestGetStatus_UntrackedFile(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	// Add an untracked file — do not stage it.
	if err := os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("untracked"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if !status.Dirty {
		t.Error("expected Dirty=true with untracked file")
	}
	if status.Untracked != 1 {
		t.Errorf("expected 1 untracked, got %d", status.Untracked)
	}
	if status.Staged != 0 {
		t.Errorf("expected 0 staged, got %d", status.Staged)
	}
}

// TestGetStatus_StagedFile verifies staged files are counted correctly.
func TestGetStatus_StagedFile(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	// Create and stage a new file.
	newFile := filepath.Join(dir, "staged.txt")
	if err := os.WriteFile(newFile, []byte("staged"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if !status.Dirty {
		t.Error("expected Dirty=true with staged file")
	}
	if status.Staged != 1 {
		t.Errorf("expected 1 staged, got %d", status.Staged)
	}
}

// TestGetStatus_ModifiedFile verifies modified (unstaged) files are counted.
func TestGetStatus_ModifiedFile(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	// Modify the file without staging.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if !status.Dirty {
		t.Error("expected Dirty=true with modified file")
	}
	if status.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", status.Modified)
	}
}

// TestGetStatus_NoUpstream verifies ahead/behind are zero when no remote is configured.
func TestGetStatus_NoUpstream(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.AheadBy != 0 {
		t.Errorf("expected AheadBy=0 with no upstream, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("expected BehindBy=0 with no upstream, got %d", status.BehindBy)
	}
}

// TestGetStatus_Cache verifies that a second call within 1 second returns the
// cached result without spawning a subprocess (cache hit is observable via timing).
func TestGetStatus_Cache(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	// First call — populates the cache.
	first := git.GetStatus(dir)
	if first == nil {
		t.Fatal("expected non-nil status on first call")
	}

	// Add an untracked file after the first call. A cache hit must return the
	// pre-change snapshot, proving no subprocess was spawned.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Second call within the TTL window — must hit the cache.
	second := git.GetStatus(dir)
	if second == nil {
		t.Fatal("expected non-nil status on second call")
	}

	// Cache hit: the new untracked file must not be visible yet.
	if second.Untracked != 0 {
		t.Errorf("expected 0 untracked from cached result (cache miss?), got %d", second.Untracked)
	}
	if second.Dirty {
		t.Errorf("expected Dirty=false from cached result, got true")
	}

	// The two calls should return the same pointer — confirming cache reuse.
	if first != second {
		t.Error("expected second call to return same pointer as first (cache hit)")
	}
}

// TestGetStatus_CacheExpiry verifies that a call after the TTL window spawns a
// fresh subprocess and returns updated results.
func TestGetStatus_CacheExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cache expiry test in short mode (requires >1s sleep)")
	}

	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "README.md", "hello")

	// Warm the cache.
	first := git.GetStatus(dir)
	if first == nil {
		t.Fatal("expected non-nil status on first call")
	}

	// Add an untracked file and wait for TTL to expire.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)

	// Call after TTL — must reflect the new file.
	second := git.GetStatus(dir)
	if second == nil {
		t.Fatal("expected non-nil status after cache expiry")
	}
	if second.Untracked != 1 {
		t.Errorf("expected 1 untracked after cache expiry, got %d", second.Untracked)
	}
}

// TestGetStatus_LineStats verifies that a dirty tree reports summed
// added/removed line counts vs HEAD (staged + unstaged).
func TestGetStatus_LineStats(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "file.txt", "a\nb\nc\n")

	// Rewrite: keep "a", drop "b" and "c", add three new lines → +3 -2.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("a\nx\ny\nz\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if !status.IsDirty() {
		t.Fatal("expected dirty tree")
	}
	if status.LinesAdded != 3 {
		t.Errorf("LinesAdded = %d, want 3", status.LinesAdded)
	}
	if status.LinesRemoved != 2 {
		t.Errorf("LinesRemoved = %d, want 2", status.LinesRemoved)
	}
}

// TestGetStatus_LineStats_CleanRepoZero verifies a clean tree reports zero
// line deltas (the numstat subprocess is skipped entirely).
func TestGetStatus_LineStats_CleanRepoZero(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "file.txt", "content\n")

	status := git.GetStatus(dir)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.LinesAdded != 0 || status.LinesRemoved != 0 {
		t.Errorf("clean repo line stats = +%d -%d, want zeros", status.LinesAdded, status.LinesRemoved)
	}
}
