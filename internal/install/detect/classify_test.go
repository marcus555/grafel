package detect

import (
	"path/filepath"
	"runtime"
	"testing"
)

// mkGitRepo creates dir/.git so dirHasGit reports true, plus an optional
// manifest file to give it a stack.
func mkGitRepo(t *testing.T, dir, manifest, body string) {
	t.Helper()
	write(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
	if manifest != "" {
		write(t, filepath.Join(dir, manifest), body)
	}
}

// TestClassifyPath_Container is the ivivo case: a plain folder holding two child
// git repos. It must report ChildGitRepos=[backend,frontend] and suggest group,
// NOT scan the parent for siblings.
func TestClassifyPath_Container(t *testing.T) {
	root := t.TempDir()
	ivivo := filepath.Join(root, "ivivo")
	mkGitRepo(t, filepath.Join(ivivo, "backend"), "go.mod", "module backend\n")
	mkGitRepo(t, filepath.Join(ivivo, "frontend"), "package.json", `{"name":"frontend"}`)
	// An unrelated sibling of ivivo that must NOT leak in.
	mkGitRepo(t, filepath.Join(root, "unrelated"), "go.mod", "module unrelated\n")

	c, err := ClassifyPath(ivivo)
	if err != nil {
		t.Fatal(err)
	}
	if c.IsGitRepo {
		t.Errorf("ivivo should not itself be a git repo")
	}
	if got := c.ChildGitRepos; len(got) != 2 || got[0] != "backend" || got[1] != "frontend" {
		t.Errorf("ChildGitRepos = %v, want [backend frontend]", got)
	}
	if c.Suggested != ActionGroup {
		t.Errorf("Suggested = %q, want group", c.Suggested)
	}
	if len(c.SiblingGitRepos) != 0 {
		t.Errorf("SiblingGitRepos should be empty for a non-repo container, got %v", c.SiblingGitRepos)
	}
}

// TestClassifyPath_SingleWithSiblings: a lone git repo that has a sibling git
// repo → single is NOT right; we want group (this repo + sibling). With no
// siblings → single.
func TestClassifyPath_SingleWithSiblings(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "service-a")
	repoB := filepath.Join(root, "service-b")
	mkGitRepo(t, repoA, "go.mod", "module a\n")
	mkGitRepo(t, repoB, "go.mod", "module b\n")

	c, err := ClassifyPath(repoA)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsGitRepo {
		t.Fatalf("service-a should be a git repo")
	}
	if c.Stack != "go" {
		t.Errorf("Stack = %q, want go", c.Stack)
	}
	if len(c.SiblingGitRepos) != 1 || filepath.Base(c.SiblingGitRepos[0]) != "service-b" {
		t.Errorf("SiblingGitRepos = %v, want [service-b]", c.SiblingGitRepos)
	}
	if c.Suggested != ActionGroup {
		t.Errorf("Suggested = %q, want group (repo with siblings)", c.Suggested)
	}
}

func TestClassifyPath_LoneSingle(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "lonely")
	mkGitRepo(t, repo, "go.mod", "module lonely\n")

	c, err := ClassifyPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SiblingGitRepos) != 0 {
		t.Errorf("SiblingGitRepos = %v, want empty", c.SiblingGitRepos)
	}
	if c.Suggested != ActionSingle {
		t.Errorf("Suggested = %q, want single", c.Suggested)
	}
}

// TestClassifyPath_SkipsHomeParentSiblingScan is the v0.1.8 TCC-prompt
// regression test for the detect side: classifying a repo that lives directly
// under $HOME must NOT enumerate the home dir for "siblings" — on macOS that
// reads INTO Desktop/Documents/Downloads and fires a permission prompt. A repo
// in ~ has no meaningful siblings, so the scan is skipped on darwin.
func TestClassifyPath_SkipsHomeParentSiblingScan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "myrepo")
	mkGitRepo(t, repo, "go.mod", "module myrepo\n")
	// A sibling git repo sitting directly in home alongside myrepo.
	mkGitRepo(t, filepath.Join(home, "othersibling"), "go.mod", "module other\n")

	c, err := ClassifyPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsGitRepo {
		t.Fatalf("myrepo should be a git repo")
	}
	if runtime.GOOS == "darwin" {
		if len(c.SiblingGitRepos) != 0 {
			t.Errorf("home-parent sibling scan must be skipped on darwin; got %v", c.SiblingGitRepos)
		}
		if c.Suggested != ActionSingle {
			t.Errorf("Suggested = %q, want single (no siblings scanned)", c.Suggested)
		}
	} else {
		// Off darwin there is no TCC, so the scan runs as before.
		if len(c.SiblingGitRepos) != 1 {
			t.Errorf("off darwin, siblings should be scanned; got %v", c.SiblingGitRepos)
		}
	}
}

// TestClassifyPath_Monorepo: a pnpm workspace → packages + monorepo action.
func TestClassifyPath_Monorepo(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "mono")
	mkGitRepo(t, repo, "pnpm-workspace.yaml", "packages:\n  - packages/*\n")
	write(t, filepath.Join(repo, "packages", "web", "package.json"), `{"name":"web"}`)
	write(t, filepath.Join(repo, "packages", "api", "package.json"), `{"name":"api"}`)

	c, err := ClassifyPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if c.Monorepo == KindNone {
		t.Fatalf("expected a monorepo kind, got none")
	}
	if len(c.Packages) != 2 {
		t.Errorf("Packages = %v, want 2 entries", c.Packages)
	}
	if c.Suggested != ActionMonorepo {
		t.Errorf("Suggested = %q, want monorepo", c.Suggested)
	}
}

// TestClassifyPath_Empty: an empty dir → no children, no packages, none.
func TestClassifyPath_Empty(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	write(t, filepath.Join(empty, ".keep"), "")

	c, err := ClassifyPath(empty)
	if err != nil {
		t.Fatal(err)
	}
	if c.IsGitRepo || len(c.ChildGitRepos) != 0 || len(c.Packages) != 0 {
		t.Errorf("empty dir misclassified: %+v", c)
	}
	if c.Suggested != ActionNone {
		t.Errorf("Suggested = %q, want none", c.Suggested)
	}
}

// TestClassifyPath_ContainerWinsOverMonorepo: child git repos take precedence
// even if a child carries a workspace manifest at the container level.
func TestClassifyPath_ContainerWinsOverMonorepo(t *testing.T) {
	root := t.TempDir()
	container := filepath.Join(root, "platform")
	mkGitRepo(t, filepath.Join(container, "api"), "go.mod", "module api\n")
	mkGitRepo(t, filepath.Join(container, "web"), "package.json", `{"name":"web"}`)

	c, err := ClassifyPath(container)
	if err != nil {
		t.Fatal(err)
	}
	if c.Suggested != ActionGroup {
		t.Errorf("Suggested = %q, want group", c.Suggested)
	}
	if len(c.ChildGitRepos) != 2 {
		t.Errorf("ChildGitRepos = %v, want 2", c.ChildGitRepos)
	}
}
