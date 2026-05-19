package daemon

import (
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/cajasmota/archigraph/internal/daemon/proto"
	"github.com/cajasmota/archigraph/internal/version"
)

// IndexFunc runs a one-shot index. The daemon does not import the
// extractor stack directly — that lives in cmd/archigraph — so it
// receives the entrypoint as a function value at construction time.
// Returning the graph.json path and the stats JSON (opaque) keeps the
// wire shape stable as the extractor evolves.
type IndexFunc func(args proto.IndexArgs) (graphPath string, statsJSON string, err error)

// RebuildFunc force-rebuilds a group. As with IndexFunc, the daemon
// stays decoupled from registry + extractor — the entrypoint is
// injected from cmd/archigraph at construction.
type RebuildFunc func(args proto.RebuildArgs) (repos []string, warning string, err error)

// Service is the RPC handler registered under proto.ServiceName. All
// public methods follow the net/rpc signature so jsonrpc can invoke
// them: func (s *Service) Method(args *T1, reply *T2) error.
//
// The Service is goroutine-safe by virtue of (a) atomic counters for
// in-flight tracking, and (b) the underlying IndexFunc/RebuildFunc
// being responsible for their own concurrency.
type Service struct {
	startedAt  time.Time
	socketPath string
	index      IndexFunc
	rebuild    RebuildFunc
	stopReq    chan<- struct{}
	stopped    int32 // atomic; 1 once stopReq has been closed
	inFlight   int64
}

// newService wires the injected entrypoints onto a fresh Service. The
// stopReq channel is closed by Stop to signal the server loop; the
// service itself never re-closes it (a stopped atomic guards the close).
func newService(idx IndexFunc, rb RebuildFunc, socketPath string, stopReq chan<- struct{}) *Service {
	return &Service{
		startedAt:  time.Now(),
		socketPath: socketPath,
		index:      idx,
		rebuild:    rb,
		stopReq:    stopReq,
	}
}

// Ping is the trivial liveness probe. Clients use it to distinguish
// "daemon not running" from "daemon running but unhealthy".
func (s *Service) Ping(_ *proto.PingArgs, reply *proto.PingReply) error {
	reply.Version = version.String()
	return nil
}

// Status reports a snapshot of daemon state. RSS is read via the Go
// runtime memstats (Sys); this is approximate but does not require
// platform-specific code. A more accurate /proc/self/status read can
// land in Phase B.
func (s *Service) Status(_ *proto.StatusArgs, reply *proto.StatusReply) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	reply.Version = version.String()
	reply.PID = os.Getpid()
	reply.UptimeSec = int64(time.Since(s.startedAt).Seconds())
	reply.RSSBytes = ms.Sys
	reply.InFlight = int(atomic.LoadInt64(&s.inFlight))
	reply.StartedAt = s.startedAt.UTC().Format(time.RFC3339)
	reply.SocketPath = s.socketPath
	return nil
}

// Index runs a single-repo index synchronously. The daemon serialises
// indexes today (Phase A) — Phase B will add a per-repo job queue and
// debouncing. Returning the stats JSON lets the client print the same
// `--json-stats` output the old standalone subcommand printed.
func (s *Service) Index(args *proto.IndexArgs, reply *proto.IndexReply) error {
	if s.index == nil {
		return errors.New("index entrypoint not configured")
	}
	if args == nil || args.RepoPath == "" {
		return errors.New("repo_path is required")
	}
	atomic.AddInt64(&s.inFlight, 1)
	defer atomic.AddInt64(&s.inFlight, -1)
	graphPath, stats, err := s.index(*args)
	if err != nil {
		return err
	}
	reply.RepoPath = args.RepoPath
	reply.GraphPath = graphPath
	reply.StatsJSON = stats
	return nil
}

// Rebuild force-indexes every repo in a group (or one slug). Wipes
// .archigraph/ first when args.Wipe is true. Cross-repo link passes
// run inside RebuildFunc so the daemon does not need to know the
// graph package.
func (s *Service) Rebuild(args *proto.RebuildArgs, reply *proto.RebuildReply) error {
	if s.rebuild == nil {
		return errors.New("rebuild entrypoint not configured")
	}
	if args == nil || args.Group == "" {
		return errors.New("group is required")
	}
	atomic.AddInt64(&s.inFlight, 1)
	defer atomic.AddInt64(&s.inFlight, -1)
	repos, warning, err := s.rebuild(*args)
	if err != nil {
		return err
	}
	reply.Repos = repos
	reply.Warning = warning
	return nil
}

// Stop initiates a graceful shutdown. The first call closes stopReq
// (signalling the server loop); later calls are no-ops. Returning
// immediately lets the client get a clean reply before the socket
// closes; the server drains in-flight work and exits.
func (s *Service) Stop(_ *proto.StopArgs, _ *proto.StopReply) error {
	if atomic.CompareAndSwapInt32(&s.stopped, 0, 1) {
		close(s.stopReq)
	}
	return nil
}
