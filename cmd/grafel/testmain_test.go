package main

// testmain_test.go — TestMain with a defensive leak detector for #2083.
//
// Before all tests run: snapshot the entry-names in this package's OWN
// isolated store (see below). After all tests complete: assert no new
// entries were created.
//
// This guard catches tests that write per-repo state to the store (i.e.
// tests that call code touching daemon.StateDirForRepo / StoreDir without
// first pinning GRAFEL_DAEMON_ROOT or GRAFEL_HOME to a temp dir) — but it
// must watch a store that belongs ONLY to this package's test binary.
//
// Historical bug (v0.1.7.4 green -> v0.1.8 red): this used to watch the
// REAL ~/.grafel/store. Under parallel `go test ./...`, a concurrent
// package's test could leak a write into the real store (because it hit
// daemon.StateDirForRepo/StoreDir without setting GRAFEL_HOME) inside this
// package's before/after window, false-failing cmd/grafel for a leak that
// happened in a completely different package. Detection was racy even
// though the underlying leaking write was deterministic.
//
// Fix: before snapshotting, redirect HOME/USERPROFILE/GRAFEL_HOME/
// GRAFEL_DAEMON_ROOT to a private temp dir for the lifetime of this test
// binary. realStoreDir() then resolves to a store that ONLY this test
// binary's own code can write to, so the detector is immune to other
// packages entirely while still catching cmd/grafel's own leaks (its
// actual purpose).
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestMain(m *testing.M) {
	tmpHome, err := os.MkdirTemp("", "grafel-cmd-grafel-testmain-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[#2083 leak-detector] failed to create isolated home: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpHome)

	isolatedGrafelHome := filepath.Join(tmpHome, ".grafel")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	os.Setenv("GRAFEL_HOME", isolatedGrafelHome)
	os.Setenv("GRAFEL_DAEMON_ROOT", isolatedGrafelHome)

	storeDir := realStoreDir()
	before := snapshotStore(storeDir)

	code := m.Run()

	after := snapshotStore(storeDir)
	leaked := diffSets(before, after)
	if len(leaked) > 0 {
		fmt.Fprintf(os.Stderr,
			"\n[#2083 leak-detector] %d new entr%s in isolated store %s after test run:\n",
			len(leaked), pluralY(len(leaked)), storeDir)
		for _, e := range leaked {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		fmt.Fprintf(os.Stderr,
			"Each leaking test must pin GRAFEL_DAEMON_ROOT (or GRAFEL_HOME) "+
				"to a t.TempDir() before calling any code that invokes daemon.StateDirForRepo.\n\n")
		// Use exit code 1 to signal failure without overriding a passing run
		// that has leaks — we report but let the caller decide.
		if code == 0 {
			code = 1
		}
	}

	os.RemoveAll(tmpHome)
	os.Exit(code)
}

// realStoreDir returns the store path the daemon would resolve given the
// current HOME/USERPROFILE. TestMain pins those to an isolated temp dir
// before calling this, so in practice this returns THIS test binary's own
// private store, not the developer's real ~/.grafel/store/.
func realStoreDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".grafel", "store")
}

// snapshotStore returns the set of top-level entry names in dir.
// Returns an empty set (not nil) when dir does not exist.
func snapshotStore(dir string) map[string]struct{} {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		set[e.Name()] = struct{}{}
	}
	return set
}

// diffSets returns names that appear in after but not in before, sorted.
func diffSets(before, after map[string]struct{}) []string {
	var leaked []string
	for name := range after {
		if _, ok := before[name]; !ok {
			leaked = append(leaked, name)
		}
	}
	sort.Strings(leaked)
	return leaked
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
