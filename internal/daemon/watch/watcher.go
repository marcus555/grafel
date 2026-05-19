package watch

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventSink is the per-repo callback invoked once a repo settles
// (i.e. no new fs events for the debounce window).
type EventSink func(repoPath string)

// Watcher is a single fsnotify-backed instance that watches one or
// more registered repos. Each repo has its own debounce timer; when
// the timer fires, the EventSink is called with the repo path.
//
// The watcher is goroutine-safe: AddRepo, RemoveRepo, and Stop may be
// called concurrently. Internal state is guarded by a single mutex
// because the volume of mutations is low (handful of repos, lifecycle
// is registration/deregistration, not per-event).
type Watcher struct {
	logger       *log.Logger
	debounce     time.Duration
	sink         EventSink
	fs           *fsnotify.Watcher
	mu           sync.Mutex
	repos        map[string]*repoState // key: absolute repo path
	dirToRepo    map[string]string     // key: absolute dir path → repo path
	stopOnce     sync.Once
	stoppedCh    chan struct{}
	totalEvents  uint64
	droppedSkips uint64
}

// repoState tracks per-repo bookkeeping. The timer is recreated on
// every event after the previous one fires; while it is non-nil and
// pending, additional events extend the window via Reset.
type repoState struct {
	path    string
	timer   *time.Timer
	pending bool
}

// NewWatcher constructs a watcher with the given debounce window. The
// sink is called once per repo per debounced burst. logger may be nil.
func NewWatcher(debounce time.Duration, sink EventSink, logger *log.Logger) (*Watcher, error) {
	if sink == nil {
		return nil, errors.New("watch: sink is required")
	}
	if logger == nil {
		logger = log.New(os.Stderr, "watch: ", log.LstdFlags)
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}
	w := &Watcher{
		logger:    logger,
		debounce:  debounce,
		sink:      sink,
		fs:        fw,
		repos:     map[string]*repoState{},
		dirToRepo: map[string]string{},
		stoppedCh: make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

// AddRepo subscribes to every directory under repoPath that survives
// the skip list. Returns the number of directories added. Idempotent:
// re-adding a registered repo is a no-op.
func (w *Watcher) AddRepo(repoPath string) (int, error) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return 0, err
	}
	w.mu.Lock()
	if _, ok := w.repos[abs]; ok {
		w.mu.Unlock()
		return 0, nil
	}
	w.repos[abs] = &repoState{path: abs}
	w.mu.Unlock()

	added := 0
	walkErr := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// Permission errors etc. — skip silently rather than abort
			// the whole subscribe.
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if p != abs && ShouldSkipDir(base) {
			return filepath.SkipDir
		}
		if err := w.fs.Add(p); err != nil {
			// inotify watch limit hit etc. — log and move on so the
			// rest of the tree is still watched.
			w.logger.Printf("watcher: add %s: %v", p, err)
			return nil
		}
		w.mu.Lock()
		w.dirToRepo[p] = abs
		w.mu.Unlock()
		added++
		return nil
	})
	if walkErr != nil {
		return added, walkErr
	}
	w.logger.Printf("watcher: registered repo=%s dirs=%d debounce=%s", abs, added, w.debounce)
	return added, nil
}

// RemoveRepo unsubscribes every directory associated with a repo. Any
// pending debounced event is cancelled.
func (w *Watcher) RemoveRepo(repoPath string) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if rs, ok := w.repos[abs]; ok {
		if rs.timer != nil {
			rs.timer.Stop()
		}
		delete(w.repos, abs)
	}
	for d, owner := range w.dirToRepo {
		if owner == abs {
			_ = w.fs.Remove(d)
			delete(w.dirToRepo, d)
		}
	}
}

// Repos returns a snapshot of the currently watched repos.
func (w *Watcher) Repos() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.repos))
	for p := range w.repos {
		out = append(out, p)
	}
	return out
}

// Stats returns coarse counters for /status output.
func (w *Watcher) Stats() (repos int, dirs int, events uint64, dropped uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.repos), len(w.dirToRepo), w.totalEvents, w.droppedSkips
}

// Stop halts the watcher and frees the fsnotify handles. Safe to call
// multiple times.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		_ = w.fs.Close()
		<-w.stoppedCh
		w.mu.Lock()
		for _, rs := range w.repos {
			if rs.timer != nil {
				rs.timer.Stop()
			}
		}
		w.mu.Unlock()
	})
}

// loop drains the fsnotify channels until the watcher is closed. We
// route every event through the per-repo debounce timer; the timer's
// callback runs the sink on its own goroutine.
func (w *Watcher) loop() {
	defer close(w.stoppedCh)
	for {
		select {
		case ev, ok := <-w.fs.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)
		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			if err != nil {
				w.logger.Printf("watcher: error: %v", err)
			}
		}
	}
}

// handleEvent classifies an fsnotify event. We do not act on Chmod-only
// events (they happen during indexing and would self-trigger). New
// directories are watched recursively so freshly-checked-out commits
// pick up nested subtrees without an explicit re-register.
func (w *Watcher) handleEvent(ev fsnotify.Event) {
	w.mu.Lock()
	w.totalEvents++
	w.mu.Unlock()

	if ev.Op == fsnotify.Chmod {
		return
	}
	if ShouldSkipPath(ev.Name) {
		w.mu.Lock()
		w.droppedSkips++
		w.mu.Unlock()
		return
	}

	// Track newly-created directories so events under them surface.
	if ev.Op.Has(fsnotify.Create) {
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			base := filepath.Base(ev.Name)
			if !ShouldSkipDir(base) {
				w.subscribeDirRecursive(ev.Name)
			}
		}
	}

	repo := w.repoFor(ev.Name)
	if repo == "" {
		return
	}
	w.armDebounce(repo)
}

// repoFor finds which registered repo a path belongs to. We walk up
// the path components and look it up in dirToRepo; if no parent dir is
// watched we treat the event as orphaned and drop it.
func (w *Watcher) repoFor(p string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir := filepath.Dir(p)
	for {
		if repo, ok := w.dirToRepo[dir]; ok {
			return repo
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// subscribeDirRecursive adds a newly-created directory (and its
// contents) to the fsnotify subscription. Used so a `git checkout`
// that creates new subtrees does not require a daemon restart.
func (w *Watcher) subscribeDirRecursive(root string) {
	repo := w.repoFor(filepath.Join(root, "_"))
	if repo == "" {
		return
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if p != root && ShouldSkipDir(base) {
			return filepath.SkipDir
		}
		if err := w.fs.Add(p); err != nil {
			return nil
		}
		w.mu.Lock()
		w.dirToRepo[p] = repo
		w.mu.Unlock()
		return nil
	})
}

// armDebounce (re)starts the per-repo timer. The timer's callback
// hands off to the sink on its own goroutine so a slow sink does not
// block the fsnotify loop.
func (w *Watcher) armDebounce(repo string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	rs, ok := w.repos[repo]
	if !ok {
		return
	}
	if rs.timer != nil {
		// Reset is safe because the timer is single-fire; if it
		// already fired the AfterFunc closure cleared rs.pending.
		rs.timer.Reset(w.debounce)
		rs.pending = true
		return
	}
	rs.pending = true
	rs.timer = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		rs := w.repos[repo]
		if rs != nil {
			rs.pending = false
		}
		w.mu.Unlock()
		w.sink(repo)
	})
}
