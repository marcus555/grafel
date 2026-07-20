package daemon

// rebuild_wait.go — serve-side "wait until the engine actually finished OUR
// rebuild, and learn whether it SUCCEEDED" for split mode (#5790, advances epic
// #5729).
//
// In split mode Service.Rebuild enqueues a KindRebuild request onto the engine
// queue and, historically, returned immediately (fire-and-forget). Callers that
// treat err==nil as "the rebuild ran" (notably `grafel group add --index`,
// which reports "indexed": true) were therefore lied to: the ack only meant the
// enqueue landed on disk, not that the engine had rebuilt anything.
//
// awaitRebuildCompletion restores an honest "err==nil means the rebuild
// SUCCEEDED" contract by reading the engine's TERMINAL ACK — the exact ack the
// request-queue consumer (ApplyAndAckBounded, from commit 6b6f18497) writes. The
// consumer is told to KEEP that ack for a WaitForCompletion request (see
// requests.ApplyAndAckBounded's keepAck and requests_drain.go's applyBucket),
// so the waiter can read its Status:
//
//   - StatusOK    → the engine ran our rebuild to completion → nil.
//   - StatusError → the engine's rebuild returned an error, OR the request was
//     dead-lettered after maxRebuildAttempts mid-apply crashes/reaps (a real
//     risk on a memory-heavy large-monorepo rebuild that OOM-reaps) → ERROR. This
//     is the honesty fix: a failed/abandoned rebuild is NEVER reported as done.
//
// Reading the ack (rather than mere request-absence) is what makes the
// success/failure distinction possible. Per-repo classification remains the
// status plane's job (rebuild_request_status.go, wizard_split_progress.go); this
// gate answers "did the engine finish, and did it succeed?".
//
// The wait is BOUNDED and failure-aware: a never-alive engine fast-fails within
// a startup window, an engine that was live and then went stale (beyond a
// GENEROUS threshold — see rebuildWaitStaleAfter) is reported as engine-death,
// and an overall timeout is the last-resort bound so a wedged engine can never
// hang the RPC forever. net/rpc gives the handler no context, so the timeout is
// the cancellation surface — identical to the pre-existing monolith synchronous
// path (rebuildRPCTimeout), which also bounds a blocking Rebuild with no ctx.

import (
	"fmt"
	"time"

	"github.com/cajasmota/grafel/internal/daemon/requests"
	"github.com/cajasmota/grafel/internal/statusfile"
)

// waitClock is the injectable time seam so tests advance/shrink time instead of
// sleeping for real poll intervals.
type waitClock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realWaitClock struct{}

func (realWaitClock) Now() time.Time        { return time.Now() }
func (realWaitClock) Sleep(d time.Duration) { time.Sleep(d) }

// Wait knobs are vars (not consts), mirroring rebuildRPCTimeout, so tests can
// shrink them to exercise the completion / timeout / engine-death paths
// deterministically; production never reassigns them.
var (
	// rebuildWaitInterval is the poll cadence between ack checks.
	rebuildWaitInterval = 500 * time.Millisecond
	// rebuildWaitStartupWindow fast-fails if the engine is NEVER seen live
	// within this window after the request was enqueued (S1 in the wizard).
	rebuildWaitStartupWindow = 30 * time.Second
	// rebuildWaitTimeout is the last-resort overall bound. Kept equal to the
	// monolith synchronous cap so both modes bound a blocking Rebuild the same,
	// and comfortably above a multi-minute large-monorepo rebuild so a healthy
	// long rebuild never false-times-out.
	rebuildWaitTimeout = rebuildRPCTimeout
	// rebuildWaitStaleAfter is how long the engine-liveness heartbeat may go
	// unrefreshed before the COMPLETION WAIT treats the engine as dead. It is
	// deliberately far more generous than the shared EngineHeartbeatStaleAfter()
	// (3×5s=15s) the doctor/warming reads use (#5790 SHOULD-FIX): a memory-heavy
	// rebuild under GC/swap pressure can starve the engine's 5s heartbeat ticker
	// for tens of seconds while the rebuild itself is perfectly healthy, and a
	// false "engine stopped responding" there would report a good rebuild as
	// not-indexed. A restart, by contrast, is NOT death for us — the request is
	// durable and the new engine redrains it (crash-resume), so its terminal ack
	// still arrives; keying death on a PID change would wrongly abandon a
	// recoverable rebuild. 2 minutes tolerates realistic GC/swap stalls yet still
	// gives up long before the 2h overall cap on a truly dead engine that is not
	// coming back.
	rebuildWaitStaleAfter = 2 * time.Minute
	// rebuildWaitClock is the injectable production clock.
	rebuildWaitClock waitClock = realWaitClock{}
	// rebuildEngineAliveFn reports whether the engine's liveness heartbeat is
	// fresh (within rebuildWaitStaleAfter). Overridable in tests; production
	// reads the engine-liveness sidecar.
	rebuildEngineAliveFn = defaultRebuildEngineAlive
)

// defaultRebuildEngineAlive reads the ambient engine-liveness heartbeat (the
// same sidecar the wizard/doctor use) and reports whether it is fresh within the
// GENEROUS completion-wait threshold rebuildWaitStaleAfter — NOT the tight
// shared EngineHeartbeatStaleAfter() default (see rebuildWaitStaleAfter's doc).
func defaultRebuildEngineAlive() bool {
	root := ""
	if layout, err := DefaultLayout(); err == nil {
		root = layout.Root
	}
	f, err := statusfile.Read(EngineLivenessStatusKey(root))
	if err != nil {
		return false
	}
	return time.Since(f.HeartbeatAt) <= rebuildWaitStaleAfter
}

// awaitRebuildCompletion blocks until the engine writes the terminal ack for the
// KindRebuild request <id> under dir, then returns nil on a StatusOK ack and an
// error on a StatusError/dead-letter ack — or a bounded failure (engine-death,
// never-alive, timeout). It is the serve-side glue Service.Rebuild calls when
// WaitForCompletion is set in split mode. The kept ack (keepAck) is consumed on
// the way out so it does not linger on disk.
func (s *Service) awaitRebuildCompletion(dir, id string) error {
	// Best-effort consume/cleanup of the kept ack regardless of outcome (a
	// no-op if the wait timed out before any ack was written).
	defer func() { _ = requests.DeleteAck(dir, id) }()
	readAck := func() (requests.Ack, bool, error) { return requests.ReadAck(dir, id) }
	return awaitRebuildAck(readAck, rebuildEngineAliveFn, rebuildWaitClock, rebuildWaitInterval, rebuildWaitStartupWindow, rebuildWaitTimeout)
}

// awaitRebuildAck is the pure completion loop (no I/O of its own beyond the
// injected closures), so it is unit-testable with fakes. It returns:
//   - nil                        — a StatusOK terminal ack appeared: engine ran our rebuild OK.
//   - "group rebuild failed" err — a StatusError/dead-letter ack appeared: honest failure.
//   - "stopped responding" err   — engine was live then went stale (death).
//   - "never became live" err    — engine was never live within startupWindow.
//   - "timed out" err            — overall bound elapsed with no terminal ack.
func awaitRebuildAck(readAck func() (requests.Ack, bool, error), alive func() bool, clk waitClock, interval, startupWindow, timeout time.Duration) error {
	start := clk.Now()
	sawAlive := false
	for {
		if ack, ok, err := readAck(); err == nil && ok {
			if ack.Status == requests.StatusError {
				reason := ack.Err
				if reason == "" {
					reason = "engine reported an error"
				}
				return fmt.Errorf("group rebuild failed: %s", reason)
			}
			return nil // StatusOK → engine finished our rebuild successfully
		}
		if alive() {
			sawAlive = true
		} else if sawAlive {
			return fmt.Errorf("index engine stopped responding before the group rebuild finished")
		}
		elapsed := clk.Now().Sub(start)
		if !sawAlive && startupWindow > 0 && elapsed >= startupWindow {
			return fmt.Errorf("index engine never became live within %s; is the daemon/engine running?", startupWindow)
		}
		if elapsed >= timeout {
			return fmt.Errorf("timed out after %s waiting for the group rebuild to finish", timeout)
		}
		clk.Sleep(interval)
	}
}
