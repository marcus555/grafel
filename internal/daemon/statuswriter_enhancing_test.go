package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/indexstate"
	"github.com/cajasmota/grafel/internal/repolock"
	"github.com/cajasmota/grafel/internal/statusfile"
)

// writeGraphFB creates a graph.fb for repo under its state dir and stamps its
// mtime, so FindGraphFile reports a controlled GraphFBMtime.
func writeGraphFB(t *testing.T, repo string, mtime time.Time) {
	t.Helper()
	stateDir := StateDirForRepo(repo)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	p := filepath.Join(stateDir, "graph.fb")
	if err := os.WriteFile(p, []byte("graph"), 0o644); err != nil {
		t.Fatalf("write graph.fb: %v", err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("chtimes graph.fb: %v", err)
	}
}

// TestWriteRepoStatusFile_IndexingVsEnhancing proves the status plane splits the
// single `indexing` flag: during EXTRACTION (foreground claim held, no queryable
// graph.fb written this run) it reports indexing=true / enhancing=false; once
// the graph becomes queryable (a graph.fb was written at/after the claim start)
// it flips to indexing=false / enhancing=true; and once the claim releases
// (index truly done) both are false.
func TestWriteRepoStatusFile_IndexingVsEnhancing(t *testing.T) {
	t.Setenv("GRAFEL_HOME", t.TempDir())
	t.Setenv("GRAFEL_DAEMON_ROOT", t.TempDir())

	repo := t.TempDir()

	// Scheduler reports CURRENT — only the foreground claim drives the split.
	indexstate.SetRepoStates([]indexstate.RepoState{
		{Path: repo, State: indexstate.StateCurrent, HeadRef: "main"},
	})
	t.Cleanup(func() { indexstate.SetRepoStates(nil) })

	// Extraction phase: claim held, no graph.fb has been (re)written this run.
	release := repolock.DefaultRegistry.ClaimForeground(repo)
	t.Cleanup(release)

	writeRepoStatusFile(repo, nil)
	got, err := statusfile.Read(repo)
	if err != nil {
		t.Fatalf("statusfile.Read: %v", err)
	}
	if !got.Indexing || got.Enhancing {
		t.Fatalf("extraction: got indexing=%v enhancing=%v, want indexing=true enhancing=false", got.Indexing, got.Enhancing)
	}

	// Graph becomes queryable: a graph.fb is written AFTER the claim started.
	writeGraphFB(t, repo, time.Now().Add(time.Second))
	writeRepoStatusFile(repo, nil)
	got, err = statusfile.Read(repo)
	if err != nil {
		t.Fatalf("statusfile.Read: %v", err)
	}
	if got.Indexing || !got.Enhancing {
		t.Fatalf("enrichment: got indexing=%v enhancing=%v, want indexing=false enhancing=true", got.Indexing, got.Enhancing)
	}

	// Index truly done: claim released.
	release()
	writeRepoStatusFile(repo, nil)
	got, err = statusfile.Read(repo)
	if err != nil {
		t.Fatalf("statusfile.Read: %v", err)
	}
	if got.Indexing || got.Enhancing {
		t.Fatalf("done: got indexing=%v enhancing=%v, want both false", got.Indexing, got.Enhancing)
	}
}

// A pre-existing (stale) graph.fb from a PRIOR run must not be mistaken for a
// queryable graph produced by the CURRENT run: while the claim is held and the
// only graph.fb on disk predates the claim start, the repo is still EXTRACTING
// (indexing=true), not enhancing.
func TestWriteRepoStatusFile_StaleGraphIsStillIndexing(t *testing.T) {
	t.Setenv("GRAFEL_HOME", t.TempDir())
	t.Setenv("GRAFEL_DAEMON_ROOT", t.TempDir())

	repo := t.TempDir()
	indexstate.SetRepoStates([]indexstate.RepoState{
		{Path: repo, State: indexstate.StateCurrent, HeadRef: "main"},
	})
	t.Cleanup(func() { indexstate.SetRepoStates(nil) })

	// An OLD graph.fb from a previous index (mtime well in the past).
	writeGraphFB(t, repo, time.Now().Add(-time.Hour))

	// New foreground index starts NOW (claim start > stale graph mtime).
	release := repolock.DefaultRegistry.ClaimForeground(repo)
	t.Cleanup(release)

	writeRepoStatusFile(repo, nil)
	got, err := statusfile.Read(repo)
	if err != nil {
		t.Fatalf("statusfile.Read: %v", err)
	}
	if !got.Indexing || got.Enhancing {
		t.Fatalf("stale graph: got indexing=%v enhancing=%v, want indexing=true enhancing=false", got.Indexing, got.Enhancing)
	}
}
