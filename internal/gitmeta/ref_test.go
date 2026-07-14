package gitmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCurrentRefFromMonorepoSubdirectory(t *testing.T) {
	repo := initGitRepo(t)
	module := filepath.Join(repo, "apps", "admin")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CurrentRef(module); got != "main" {
		t.Fatalf("CurrentRef(module) = %q, want main", got)
	}
}

func TestCurrentRefDetachedHead(t *testing.T) {
	repo := initGitRepo(t)
	cmd := exec.Command("git", "checkout", "--detach", "HEAD")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("detach HEAD: %v\n%s", err, out)
	}
	if got := CurrentRef(repo); got != "" {
		t.Fatalf("CurrentRef(detached) = %q, want empty", got)
	}
}

func TestCurrentRefLinkedWorktree(t *testing.T) {
	repo := initGitRepo(t)
	worktree := filepath.Join(t.TempDir(), "feature-worktree")
	cmd := exec.Command("git", "worktree", "add", "-b", "feature/cache", worktree)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add worktree: %v\n%s", err, out)
	}
	if got := CurrentRef(filepath.Join(worktree, "nested")); got != "feature/cache" {
		t.Fatalf("CurrentRef(worktree) = %q, want feature/cache", got)
	}
}

func TestCurrentRefNonGitPath(t *testing.T) {
	if got := CurrentRef(t.TempDir()); got != "" {
		t.Fatalf("CurrentRef(non-git) = %q, want empty", got)
	}
}
