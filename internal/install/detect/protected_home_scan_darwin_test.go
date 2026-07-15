// protected_home_scan_darwin_test.go — regression repro for the macOS TCC
// batch-prompt bug: classifying $HOME descended INTO its TCC-protected children
// (Documents/Downloads/Pictures/Music/…), firing a batch of permission prompts.
//
// ClassifyPath runs childGitRepoNames(home) — which Stats each child's .git —
// and DetectMonorepo(home) → scanPolyglotModules — which ReadDir's each child
// looking for manifests/source. Both READ INTO the protected home children. The
// v0.1.8 sibling-scan guard only covered siblingGitRepos, not these two paths.
//
// The test builds a FAKE home (never the real ~) whose protected children carry
// planted markers (.git dirs, ecosystem manifests). If the classifier descends
// into a protected child, that child surfaces in ChildGitRepos / Packages — the
// observable proof of a TCC-tripping descent. The guard must keep them empty
// while leaving non-protected children (~/myrepo) fully classified.
//
// darwin-only: off macOS there is no TCC and the descent is correct behaviour.

//go:build darwin

package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// protectedNames is the TCC-protected set that sits directly under $HOME.
var protectedNames = []string{
	"Desktop", "Documents", "Downloads", "Library",
	"Movies", "Music", "Pictures", "Public",
}

// buildFakeHome creates an isolated fake $HOME whose protected children each
// look "indexable" (a .git dir AND a go.mod), plus a non-protected direct child
// git repo (~/myrepo) that MUST still be discovered. Returns the fake home path.
func buildFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()

	writeFile := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	mkdir := func(path string) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}

	// Protected children: planted so that ANY descent is observable.
	for _, name := range protectedNames {
		child := filepath.Join(home, name)
		mkdir(filepath.Join(child, ".git"))                   // trips childGitRepoNames
		writeFile(filepath.Join(child, "go.mod"), "module x") // trips scanPolyglotModules
		writeFile(filepath.Join(child, "main.go"), "package x")
	}

	// A legitimate, non-protected direct child git repo that must remain
	// discoverable (the guard must not throw out the baby with the bathwater).
	direct := filepath.Join(home, "myrepo")
	mkdir(filepath.Join(direct, ".git"))

	return home
}

// hitProtected returns the members of got that name a protected home child.
func hitProtected(got []string) []string {
	set := map[string]bool{}
	for _, n := range protectedNames {
		set[n] = true
	}
	var hit []string
	for _, g := range got {
		// Packages/ChildGitRepos may be names or relative paths; match the head.
		if set[g] || set[filepath.Base(g)] {
			hit = append(hit, g)
		}
	}
	return hit
}

// TestClassifyHome_DoesNotDescendIntoProtectedChildren is the core repro. Before
// the guard, ClassifyPath(home) reports the protected children as ChildGitRepos
// (Stat'd .git) and/or Packages (ReadDir'd manifests) — proving it read INTO
// them. After the guard, both lists must be free of any protected child while
// the non-protected ~/myrepo child git repo is still found.
func TestClassifyHome_DoesNotDescendIntoProtectedChildren(t *testing.T) {
	home := buildFakeHome(t)
	t.Setenv("HOME", home)

	got, err := ClassifyPath(home)
	if err != nil {
		t.Fatalf("ClassifyPath(home): %v", err)
	}

	if hit := hitProtected(got.ChildGitRepos); len(hit) > 0 {
		t.Errorf("ChildGitRepos descended into protected home children %v (probed each .git — TCC batch prompt)", hit)
	}
	if hit := hitProtected(got.Packages); len(hit) > 0 {
		t.Errorf("Packages descended into protected home children %v (ReadDir'd each for manifests — TCC batch prompt)", hit)
	}

	// The guard must NOT throw out the baby: a non-protected child git repo under
	// home is still legitimately discoverable.
	foundMyrepo := false
	for _, c := range got.ChildGitRepos {
		if c == "myrepo" {
			foundMyrepo = true
		}
	}
	if !foundMyrepo {
		t.Errorf("non-protected child git repo 'myrepo' was dropped; ChildGitRepos=%v", got.ChildGitRepos)
	}
}

// TestClassifyProtectedFolderDirectly_StillDescends guards the deliberate case:
// when the user explicitly points the classifier AT a protected folder
// (~/Documents itself), reading its CONTENTS is intended — the guard only skips
// protected folders reached as CHILDREN of $HOME, not when they are the root.
func TestClassifyProtectedFolderDirectly_StillDescends(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	docs := filepath.Join(home, "Documents")
	for _, s := range []string{"svc-a", "svc-b"} {
		if err := os.MkdirAll(filepath.Join(docs, s), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(docs, s, "go.mod"), []byte("module "+s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ClassifyPath(docs)
	if err != nil {
		t.Fatalf("ClassifyPath(Documents): %v", err)
	}
	// Explicitly classifying ~/Documents should still see its children (svc-a,
	// svc-b) as monorepo packages — the guard must not neuter explicit selection.
	if len(got.Packages) == 0 {
		t.Errorf("explicit classify of ~/Documents found no packages; guard over-reached. got=%+v", got)
	}
}
