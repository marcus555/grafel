package gitmeta

import (
	"os"
	"path/filepath"
	"strings"
)

// CurrentRef returns the current local branch name without spawning git. It
// walks upward from repoPath so module directories inside a monorepo resolve
// through the shared root .git entry. Detached HEAD and non-git paths return
// "", matching Capture(repoPath).Ref.
func CurrentRef(repoPath string) string {
	headPath := findHeadFile(repoPath)
	if headPath == "" {
		return ""
	}
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	const prefix = "ref: refs/heads/"
	head := strings.TrimSpace(string(data))
	if !strings.HasPrefix(head, prefix) {
		return ""
	}
	return strings.TrimPrefix(head, prefix)
}

// findHeadFile locates the worktree-specific HEAD file for a repository root
// or any directory below it. A linked worktree has a .git indirection file;
// a normal checkout has a .git directory.
func findHeadFile(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	dir, err := filepath.Abs(repoPath)
	if err != nil {
		dir = repoPath
	}
	dir = filepath.Clean(dir)
	if info, statErr := os.Stat(dir); statErr == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		dotGit := filepath.Join(dir, ".git")
		if info, statErr := os.Stat(dotGit); statErr == nil {
			if info.IsDir() {
				return filepath.Join(dotGit, "HEAD")
			}
			if gitDir := readGitdirFile(dotGit); gitDir != "" {
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(dir, gitDir)
				}
				return filepath.Join(filepath.Clean(gitDir), "HEAD")
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
