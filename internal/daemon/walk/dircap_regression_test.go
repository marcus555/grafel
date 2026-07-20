package walk

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestWalkRepo_LargeLegitMonorepoFullyWalked is the regression test for the
// dir-cap truncation bug: a synthetic tree with MORE directories than the
// OLD DefaultWatchDirCap (5000) but FEWER than the new, raised cap must be
// walked in full — every leaf file returned, none dropped with rule
// "dir-cap". Before the fix (cap=5000) this test fails because
// alphabetically-late subtrees never get walked. After raising the cap it
// passes.
//
// We keep the tree cheap: one file per directory, flat siblings under root
// (no deep nesting), so creating ~6000 dirs is fast and nothing is committed
// to the repo (t.TempDir()).
func TestWalkRepo_LargeLegitMonorepoFullyWalked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large synthetic-tree walk in -short mode")
	}

	root := t.TempDir()

	const dirCount = 6000 // > old DefaultWatchDirCap (5000), < new cap
	var want []string
	for i := 0; i < dirCount; i++ {
		rel := filepath.Join(fmt.Sprintf("pkg%05d", i), "f.go")
		mkSrc(t, filepath.Join(root, rel))
		want = append(want, filepath.ToSlash(rel))
	}
	// An alphabetically-late subtree, analogous to domains/platform/... in
	// the real corpus, which was the concrete symptom of the bug.
	lateRel := filepath.Join("zzz-late-subtree", "z.go")
	mkSrc(t, filepath.Join(root, lateRel))
	want = append(want, filepath.ToSlash(lateRel))

	var skipLog bytes.Buffer
	files, skipped, err := WalkRepo(root, &Options{PrintSkipped: &skipLog})
	if err != nil {
		t.Fatalf("WalkRepo: %v", err)
	}

	for _, w := range want {
		if !contains(files, w) {
			t.Errorf("expected %q to be walked (dir-cap must not truncate a legit large monorepo), got %d files total", w, len(files))
		}
	}

	for _, s := range skipped {
		if s.Rule == "dir-cap" {
			t.Errorf("expected no dir-cap skips for a %d-directory tree under the (raised) cap, got skip: %+v", dirCount, s)
		}
	}
	if strings.Contains(skipLog.String(), "exceeded the") {
		t.Errorf("expected no dir-cap WARN for a %d-directory tree under the (raised) cap, got log: %q", dirCount, skipLog.String())
	}
}

// TestWatchDirCap_EnvOverrideStillTruncates confirms the GRAFEL_WATCH_DIR_CAP
// override still works after raising the default: setting it low resumes
// truncation with a WARN, proving the cap mechanism itself is intact.
func TestWatchDirCap_EnvOverrideStillTruncates(t *testing.T) {
	t.Setenv("GRAFEL_WATCH_DIR_CAP", "5")
	root := t.TempDir()
	for i := 0; i < 30; i++ {
		mkSrc(t, filepath.Join(root, fmt.Sprintf("d%03d", i), "f.go"))
	}

	var skipLog bytes.Buffer
	files, skipped, err := WalkRepo(root, &Options{PrintSkipped: &skipLog})
	if err != nil {
		t.Fatalf("WalkRepo: %v", err)
	}
	if len(files) >= 30 {
		t.Errorf("env override cap=5 should still truncate, got %d files", len(files))
	}
	var sawCapSkip bool
	for _, s := range skipped {
		if s.Rule == "dir-cap" {
			sawCapSkip = true
		}
	}
	if !sawCapSkip {
		t.Errorf("expected a dir-cap skip entry with low env override, got %+v", skipped)
	}
	if !strings.Contains(skipLog.String(), "WARN") {
		t.Errorf("expected a dir-cap WARN with low env override, got %q", skipLog.String())
	}
}

// TestWalkRepo_ExceedsNewCapStillWarnsAndTruncates proves the cap mechanism
// is still intact at the new (raised) default: a tree that exceeds even the
// new cap must still warn once and truncate the remaining subtrees.
func TestWalkRepo_ExceedsNewCapStillWarnsAndTruncates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large synthetic-tree walk in -short mode")
	}
	// Use a small local override so the test doesn't need to create
	// DefaultWatchDirCap+1 real directories to exercise "exceeds the cap".
	t.Setenv("GRAFEL_WATCH_DIR_CAP", "50")
	root := t.TempDir()
	for i := 0; i < 100; i++ {
		mkSrc(t, filepath.Join(root, fmt.Sprintf("d%03d", i), "f.go"))
	}

	var skipLog bytes.Buffer
	files, skipped, err := WalkRepo(root, &Options{PrintSkipped: &skipLog})
	if err != nil {
		t.Fatalf("WalkRepo: %v", err)
	}
	if len(files) >= 100 {
		t.Errorf("exceeding the cap should still truncate, got %d files", len(files))
	}
	var sawCapSkip bool
	for _, s := range skipped {
		if s.Rule == "dir-cap" {
			sawCapSkip = true
		}
	}
	if !sawCapSkip {
		t.Errorf("expected a dir-cap skip entry when exceeding the cap, got %+v", skipped)
	}
	if !strings.Contains(skipLog.String(), "WARN") || !strings.Contains(skipLog.String(), "cap") {
		t.Errorf("expected a dir-cap WARN when exceeding the cap, got %q", skipLog.String())
	}
}
