// Package sched is the daemon's reactive scheduler (Phase B). The
// watcher hands off settled-repo notifications to Enqueue; the
// scheduler serialises per-repo indexes, runs them on a small worker
// pool, then schedules:
//
//   - a debounced cross-repo link recompute per group (10s),
//   - a debounced graph-algorithm pass per repo (30s),
//
// both of which are cancelled and rescheduled if new write activity
// arrives during the window. The link recompute and algorithm pass
// run via caller-supplied callbacks so the scheduler stays free of
// extractor + graph package dependencies.
package sched

import (
	"context"
	"log"
	"os"
	"sync"
	"time"
)

// IndexFn re-indexes a single repo. The scheduler invokes it on a
// worker goroutine; concurrent calls for distinct repos may run in
// parallel up to the worker-pool size, but each repo path is
// serialised against itself.
type IndexFn func(ctx context.Context, repoPath string) error

// LinksFn re-runs the cross-repo link passes for a group.
type LinksFn func(ctx context.Context, group string) error

// AlgoFn runs the graph-algorithm pass for a repo (community detection,
// PageRank, articulation points). It is scheduled after a successful
// index settles and is cancelled+rescheduled on any further write.
type AlgoFn func(ctx context.Context, repoPath string) error

// GroupsForRepoFn returns the group names a repo participates in.
// Provided by the caller so the scheduler does not import the registry.
type GroupsForRepoFn func(repoPath string) []string

// Config wires the scheduler. All function fields are required; nil
// causes Enqueue to short-circuit with a logged warning.
type Config struct {
	Workers       int           // worker pool size; defaults to 2
	LinkDebounce  time.Duration // group settling window; defaults to 10s
	AlgoDebounce  time.Duration // per-repo algo delay; defaults to 30s
	Index         IndexFn
	Links         LinksFn
	Algorithms    AlgoFn
	GroupsForRepo GroupsForRepoFn
	Logger        *log.Logger
}

// Scheduler is constructed once per daemon. It owns:
//   - a bounded job channel (per-repo dedup happens before enqueue),
//   - a worker pool,
//   - per-group link debounce timers,
//   - per-repo algorithm debounce timers.
type Scheduler struct {
	cfg    Config
	logger *log.Logger
	jobs   chan string // repo paths to index
	enq    chan string // public enqueue input → dedup → jobs
	stop   chan struct{}
	wg     sync.WaitGroup

	mu           sync.Mutex
	inflight     map[string]bool // repos currently indexing
	pendingIndex map[string]bool // repos already enqueued but not yet indexing
	queueLen     int             // length of enq+jobs (approx, for status)
	linkTimers   map[string]*time.Timer
	linkPending  map[string]bool
	algoTimers   map[string]*time.Timer
	algoPending  map[string]bool
	algoCancel   map[string]context.CancelFunc
	indexedRepos map[string]repoStats
	recentLog    []LogEntry
}

// repoStats records what we know about each successful index pass.
type repoStats struct {
	LastIndex  time.Time
	LastAlgo   time.Time
	IndexCount int64
	AlgoCount  int64
	LastErr    string
}

// LogEntry is a single structured event captured for /status. Kept in
// memory only; the daemon's regular log file remains authoritative.
type LogEntry struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	Repo string    `json:"repo,omitempty"`
	Msg  string    `json:"msg"`
}

const maxRecentLog = 32

// New constructs a scheduler. Start must be called before Enqueue.
func New(cfg Config) *Scheduler {
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.LinkDebounce <= 0 {
		cfg.LinkDebounce = 10 * time.Second
	}
	if cfg.AlgoDebounce <= 0 {
		cfg.AlgoDebounce = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stderr, "sched: ", log.LstdFlags)
	}
	return &Scheduler{
		cfg:          cfg,
		logger:       cfg.Logger,
		jobs:         make(chan string, 64),
		enq:          make(chan string, 64),
		stop:         make(chan struct{}),
		inflight:     map[string]bool{},
		pendingIndex: map[string]bool{},
		linkTimers:   map[string]*time.Timer{},
		linkPending:  map[string]bool{},
		algoTimers:   map[string]*time.Timer{},
		algoPending:  map[string]bool{},
		algoCancel:   map[string]context.CancelFunc{},
		indexedRepos: map[string]repoStats{},
	}
}

// Start spins up the dedup goroutine + worker pool. Stop reverses it.
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.dedupLoop()
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.workerLoop()
	}
}

// Stop closes the channels and waits for in-flight work to drain.
func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.linkTimers {
		t.Stop()
	}
	for _, t := range s.algoTimers {
		t.Stop()
	}
	for _, c := range s.algoCancel {
		c()
	}
}

// Enqueue requests a (debounced+deduped) reindex of repoPath. Safe to
// call from arbitrary goroutines.
func (s *Scheduler) Enqueue(repoPath string) {
	select {
	case s.enq <- repoPath:
	case <-s.stop:
	}
}

// dedupLoop forwards from enq to jobs, suppressing duplicate enqueues
// for repos that are already pending or running. This is also where
// we cancel any scheduled algorithm pass — any new write activity in
// the repo invalidates the pending algo schedule.
func (s *Scheduler) dedupLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		case p := <-s.enq:
			s.mu.Lock()
			// Cancel any pending algorithm pass for this repo; new
			// writes mean the algo pass would race against the
			// upcoming index.
			s.cancelAlgoLocked(p)
			if s.inflight[p] || s.pendingIndex[p] {
				s.mu.Unlock()
				continue
			}
			s.pendingIndex[p] = true
			s.queueLen++
			s.mu.Unlock()
			select {
			case s.jobs <- p:
			case <-s.stop:
				return
			}
		}
	}
}

// workerLoop pulls jobs off the channel and runs them under a per-repo
// serialisation lock. Two workers means two repos can index in parallel.
func (s *Scheduler) workerLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		case p, ok := <-s.jobs:
			if !ok {
				return
			}
			s.runIndex(p)
		}
	}
}

// runIndex is the per-job wrapper: serialise against same-repo runs,
// invoke IndexFn, then schedule the downstream link + algo passes.
func (s *Scheduler) runIndex(repoPath string) {
	s.mu.Lock()
	s.pendingIndex[repoPath] = false
	s.queueLen--
	if s.inflight[repoPath] {
		// Already running — push back onto the queue.
		s.mu.Unlock()
		go func() {
			time.Sleep(50 * time.Millisecond)
			s.Enqueue(repoPath)
		}()
		return
	}
	s.inflight[repoPath] = true
	s.mu.Unlock()

	t0 := time.Now()
	s.logEvent("index_start", repoPath, "")
	var err error
	if s.cfg.Index != nil {
		err = s.cfg.Index(context.Background(), repoPath)
	}
	s.mu.Lock()
	stats := s.indexedRepos[repoPath]
	stats.LastIndex = time.Now()
	stats.IndexCount++
	if err != nil {
		stats.LastErr = err.Error()
	} else {
		stats.LastErr = ""
	}
	s.indexedRepos[repoPath] = stats
	s.inflight[repoPath] = false
	s.mu.Unlock()

	if err != nil {
		s.logEvent("index_err", repoPath, err.Error())
		s.logger.Printf("sched: index %s failed: %v (took %s)", repoPath, err, time.Since(t0))
		return
	}
	s.logEvent("index_ok", repoPath, time.Since(t0).Truncate(time.Millisecond).String())

	// Schedule downstream passes.
	s.scheduleAlgo(repoPath)
	if s.cfg.GroupsForRepo != nil {
		for _, g := range s.cfg.GroupsForRepo(repoPath) {
			s.scheduleLinks(g)
		}
	}
}

// scheduleLinks (re)arms the per-group link debounce timer. The 10s
// window is meant to coalesce bursts where multiple repos in a group
// re-index back-to-back.
func (s *Scheduler) scheduleLinks(group string) {
	if s.cfg.Links == nil {
		return
	}
	s.mu.Lock()
	if t, ok := s.linkTimers[group]; ok {
		t.Stop()
	}
	s.linkPending[group] = true
	s.linkTimers[group] = time.AfterFunc(s.cfg.LinkDebounce, func() {
		s.mu.Lock()
		s.linkPending[group] = false
		delete(s.linkTimers, group)
		s.mu.Unlock()
		s.runLinks(group)
	})
	s.mu.Unlock()
}

func (s *Scheduler) runLinks(group string) {
	s.logEvent("links_start", "", group)
	t0 := time.Now()
	err := s.cfg.Links(context.Background(), group)
	if err != nil {
		s.logEvent("links_err", "", group+": "+err.Error())
		s.logger.Printf("sched: links %s failed: %v", group, err)
		return
	}
	s.logEvent("links_ok", "", group+" "+time.Since(t0).Truncate(time.Millisecond).String())
}

// scheduleAlgo (re)arms the per-repo algorithm pass timer. Any pending
// pass is cancelled first; a new pass starts the 30s window over.
func (s *Scheduler) scheduleAlgo(repoPath string) {
	if s.cfg.Algorithms == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelAlgoLocked(repoPath)
	s.algoPending[repoPath] = true
	s.algoTimers[repoPath] = time.AfterFunc(s.cfg.AlgoDebounce, func() {
		s.mu.Lock()
		s.algoPending[repoPath] = false
		delete(s.algoTimers, repoPath)
		ctx, cancel := context.WithCancel(context.Background())
		s.algoCancel[repoPath] = cancel
		s.mu.Unlock()

		s.runAlgo(ctx, repoPath)

		s.mu.Lock()
		delete(s.algoCancel, repoPath)
		s.mu.Unlock()
	})
}

// cancelAlgoLocked stops any pending timer or cancels an in-flight
// algorithm pass for the given repo. MUST be called with s.mu held.
func (s *Scheduler) cancelAlgoLocked(repoPath string) {
	if t, ok := s.algoTimers[repoPath]; ok {
		t.Stop()
		delete(s.algoTimers, repoPath)
		s.algoPending[repoPath] = false
	}
	if c, ok := s.algoCancel[repoPath]; ok {
		c()
		delete(s.algoCancel, repoPath)
	}
}

func (s *Scheduler) runAlgo(ctx context.Context, repoPath string) {
	s.logEvent("algo_start", repoPath, "")
	t0 := time.Now()
	err := s.cfg.Algorithms(ctx, repoPath)
	if err != nil {
		if ctx.Err() != nil {
			s.logEvent("algo_cancelled", repoPath, "")
			return
		}
		s.logEvent("algo_err", repoPath, err.Error())
		s.logger.Printf("sched: algo %s failed: %v", repoPath, err)
		return
	}
	s.mu.Lock()
	stats := s.indexedRepos[repoPath]
	stats.LastAlgo = time.Now()
	stats.AlgoCount++
	s.indexedRepos[repoPath] = stats
	s.mu.Unlock()
	s.logEvent("algo_ok", repoPath, time.Since(t0).Truncate(time.Millisecond).String())
}

// Snapshot reports current scheduler state for the Status RPC.
type Snapshot struct {
	QueueLen     int
	InFlight     []string
	PendingAlgo  []string
	PendingLinks []string
	IndexedRepos []RepoSnapshot
	RecentLog    []LogEntry
}

// RepoSnapshot is one repo's slice of Snapshot.
type RepoSnapshot struct {
	Path       string    `json:"path"`
	LastIndex  time.Time `json:"last_index"`
	LastAlgo   time.Time `json:"last_algo"`
	IndexCount int64     `json:"index_count"`
	AlgoCount  int64     `json:"algo_count"`
	LastErr    string    `json:"last_err,omitempty"`
}

// Snapshot returns a defensive copy of the scheduler's user-visible
// state. Safe to call from the RPC handler.
func (s *Scheduler) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{
		QueueLen: s.queueLen,
	}
	for p := range s.inflight {
		if s.inflight[p] {
			out.InFlight = append(out.InFlight, p)
		}
	}
	for p := range s.algoPending {
		if s.algoPending[p] {
			out.PendingAlgo = append(out.PendingAlgo, p)
		}
	}
	for g := range s.linkPending {
		if s.linkPending[g] {
			out.PendingLinks = append(out.PendingLinks, g)
		}
	}
	for p, st := range s.indexedRepos {
		out.IndexedRepos = append(out.IndexedRepos, RepoSnapshot{
			Path: p, LastIndex: st.LastIndex, LastAlgo: st.LastAlgo,
			IndexCount: st.IndexCount, AlgoCount: st.AlgoCount,
			LastErr: st.LastErr,
		})
	}
	if n := len(s.recentLog); n > 0 {
		out.RecentLog = append(out.RecentLog, s.recentLog...)
	}
	return out
}

// MarkIndexed lets the daemon record a non-watcher-driven index (e.g.
// an explicit `archigraph index` RPC) so Status reflects reality.
func (s *Scheduler) MarkIndexed(repoPath string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.indexedRepos[repoPath]
	stats.LastIndex = time.Now()
	stats.IndexCount++
	if err != nil {
		stats.LastErr = err.Error()
	} else {
		stats.LastErr = ""
	}
	s.indexedRepos[repoPath] = stats
}

// logEvent appends to the in-memory recent-log buffer (capped at
// maxRecentLog). The daemon log file remains the authoritative store.
func (s *Scheduler) logEvent(kind, repo, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recentLog = append(s.recentLog, LogEntry{Time: time.Now(), Kind: kind, Repo: repo, Msg: msg})
	if len(s.recentLog) > maxRecentLog {
		s.recentLog = s.recentLog[len(s.recentLog)-maxRecentLog:]
	}
}
