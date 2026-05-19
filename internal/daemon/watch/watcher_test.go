package watch

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShouldSkipDir(t *testing.T) {
	cases := map[string]bool{
		"node_modules": true,
		".git":         true,
		"target":       true,
		"src":          false,
		"internal":     false,
		".archigraph":  true,
		"dist":         true,
	}
	for in, want := range cases {
		if got := ShouldSkipDir(in); got != want {
			t.Errorf("ShouldSkipDir(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestShouldSkipPath(t *testing.T) {
	cases := map[string]bool{
		"/repo/src/main.go":                false,
		"/repo/node_modules/foo/bar.js":    true,
		"/repo/.git/HEAD":                  true,
		"/repo/target/build.out":           true,
		"/repo/src/.archigraph/graph.json": true,
		"/repo/a.log":                      true,
		"/repo/a.swp":                      true,
		"/repo/cmd/foo/main_test.go":       false,
	}
	for in, want := range cases {
		if got := ShouldSkipPath(in); got != want {
			t.Errorf("ShouldSkipPath(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestDebounce verifies that a burst of writes within the debounce
// window collapses to a single sink invocation.
func TestDebounce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	repo := t.TempDir()
	src := filepath.Join(repo, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	doneCh := make(chan string, 4)
	w, err := NewWatcher(150*time.Millisecond, func(repoPath string) {
		calls.Add(1)
		doneCh <- repoPath
	}, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Stop()
	if _, err := w.AddRepo(repo); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Burst of 5 writes within ~50ms — fsnotify will surface each as
	// a Write event but the debouncer should coalesce them.
	for i := 0; i < 5; i++ {
		f := filepath.Join(src, "main.go")
		if err := os.WriteFile(f, []byte("package main // burst"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("debounce did not fire within 2s; calls=%d", calls.Load())
	}

	// Wait long enough that any leftover timer would have fired.
	time.Sleep(400 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("expected single debounced fire, got %d", got)
	}
}

// TestDebounceTwoBursts verifies that two separate bursts (separated
// by more than the debounce window) each trigger one sink call.
func TestDebounceTwoBursts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	var (
		mu    sync.Mutex
		calls int
	)
	w, err := NewWatcher(100*time.Millisecond, func(string) {
		mu.Lock()
		calls++
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Stop()
	if _, err := w.AddRepo(repo); err != nil {
		t.Fatalf("add: %v", err)
	}

	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "src", name),
			[]byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("a.go")
	time.Sleep(300 * time.Millisecond) // exceed debounce window
	touch("b.go")
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 2 {
		t.Errorf("two bursts should yield two sink calls, got %d", got)
	}
}

// TestSkipDirRespected verifies that creating files inside a skipped
// directory does NOT trigger the sink.
func TestSkipDirRespected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	repo := t.TempDir()
	nm := filepath.Join(repo, "node_modules", "foo")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repo, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	w, err := NewWatcher(100*time.Millisecond, func(string) {
		calls.Add(1)
	}, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Stop()
	if _, err := w.AddRepo(repo); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Write inside node_modules — should be ignored.
	if err := os.WriteFile(filepath.Join(nm, "ignored.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Errorf("expected 0 sink calls for node_modules write, got %d", got)
	}

	// Sanity check: write to src — should trigger.
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 sink call for src write, got %d", got)
	}
}
