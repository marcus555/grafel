package daemon

// Regression tests for the ADR-0024 drain-loop starvation + rebuild-runaway
// defects (epic #5729, split mode ON):
//
//   Fix 1 (blocker): a cheap KindReindex must be drained/enqueued even while an
//     EXPENSIVE, multi-minute KindRebuild is in-flight. The old drain applied
//     KindRebuild INLINE on the single drain goroutine, so a running rebuild
//     starved every other group/repo's reindex until it finished.
//   Fix 2a (runaway): N rapid `grafel rebuild` for the SAME group each write a
//     distinct-UUID KindRebuild file; they must COALESCE into exactly one
//     rebuildFn call, not N sequential full-group rebuilds.

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon/proto"
	"github.com/cajasmota/grafel/internal/daemon/requests"
	"github.com/cajasmota/grafel/internal/daemon/sched"
)

// Fix 1: with a KindRebuild whose rebuildFn blocks (the in-process analogue of
// a multi-minute rebuild) ordered BEFORE a KindReindex in the same dir, the
// reindex must still reach the scheduler. The old inline-rebuild drain blocks
// on the rebuild and never processes the reindex — this test times out (red).
func TestDrainRequestsOnce_ReindexNotBlockedByInflightRebuild(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	repoPath := t.TempDir()
	dir := requestsDirForRepo(repoPath)

	// Rebuild first (older CreatedAt → ListPending returns it before the
	// reindex), so the OLD code hits the inline rebuild before the reindex.
	const group = "blocking-group"
	payload, err := json.Marshal(proto.RebuildArgs{Group: group})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := requests.Write(dir, requests.Record{
		Kind:      requests.KindRebuild,
		Payload:   payload,
		CreatedAt: time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatalf("Write rebuild: %v", err)
	}
	if _, err := requests.Write(dir, requests.Record{
		Kind:      requests.KindReindex,
		RepoPath:  repoPath,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Write reindex: %v", err)
	}

	block := make(chan struct{})
	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		<-block // hold the rebuild "in-flight"
		return []string{repoPath}, "", nil
	}

	indexed := make(chan string, 1)
	sc := sched.New(sched.Config{
		Index: func(ctx context.Context, repo, ref string) error {
			indexed <- repo
			return nil
		},
	})
	sc.Start()
	defer sc.Stop()

	done := make(chan error, 1)
	go func() { done <- drainRequestsOnce(requestsRoot(), sc, rebuildFn, nil) }()

	select {
	case repo := <-indexed:
		if repo != repoPath {
			t.Fatalf("indexed wrong repo: got %q want %q", repo, repoPath)
		}
	case <-time.After(3 * time.Second):
		close(block)
		<-done
		t.Fatal("reindex was NOT enqueued while a rebuild was in-flight — drain loop is blocked by the inline rebuild")
	}

	close(block) // let the background rebuild finish
	<-done
}

// Fix 2a: N identical same-group KindRebuild requests coalesce into exactly one
// rebuildFn call. The old drain applied each pending rebuild inline → N calls.
func TestDrainRequestsOnce_CoalescesDuplicateSameGroupRebuilds(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "coalesce-group"
	dir := requestsDirForGroup(group)
	const n = 5
	for i := 0; i < n; i++ {
		payload, err := json.Marshal(proto.RebuildArgs{Group: group})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload}); err != nil {
			t.Fatalf("Write: %v", err)
		}
		time.Sleep(2 * time.Millisecond) // distinct CreatedAt for a stable order
	}

	var calls int32
	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		atomic.AddInt32(&calls, 1)
		return []string{"/repo"}, "", nil
	}

	if err := drainRequestsOnce(requestsRoot(), nil, rebuildFn, nil); err != nil {
		t.Fatalf("drainRequestsOnce: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected %d duplicate same-group rebuilds to coalesce into exactly 1 rebuildFn call, got %d", n, got)
	}
	recs, err := requests.ListPending(dir)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected all coalesced requests consumed, still pending: %d", len(recs))
	}
}

// Fix 2b: a rebuild whose apply already crashed once (Attempts>=1) must NOT be
// re-applied on the very next drain tick — the crash-resume re-apply is gated
// by a growing backoff keyed on the attempt count. Only after the backoff
// window elapses does a later drain re-apply. Uses a persistent drainer (the
// production shape) with an injected clock + shrunk backoff for determinism.
func TestRebuildWorker_CrashResumeBackoffGate(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "backoff-group"
	dir := requestsDirForGroup(group)
	payload, err := json.Marshal(proto.RebuildArgs{Group: group})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var calls int32
	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		atomic.AddInt32(&calls, 1)
		panic("simulated crash mid-rebuild")
	}

	now := time.Now()
	d := newRequestsDrainer(nil, rebuildFn, nil)
	d.rebuilds.now = func() time.Time { return now }
	d.rebuilds.backoff = func(int) time.Duration { return time.Minute }

	// Drain #1: first apply (Attempts 0→1) crashes; schedules a backoff.
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce #1: %v", err)
	}
	d.rebuilds.waitIdle()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("first drain should apply exactly once, got %d", got)
	}

	// Drain #2, same wall clock: within the backoff window → must NOT re-apply.
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce #2: %v", err)
	}
	d.rebuilds.waitIdle()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("second drain within backoff window must NOT re-apply (back-to-back re-apply on every tick), got %d", got)
	}

	// Advance past the backoff window: a later drain re-applies (attempt 2).
	now = now.Add(2 * time.Minute)
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce #3: %v", err)
	}
	d.rebuilds.waitIdle()
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("after backoff elapses the drain should re-apply once more, got %d", got)
	}
}

// Fix 2c: a rebuild that exhausts its attempt budget is DEAD-LETTERED — the
// request file is gone, but an OBSERVABLE dead-letter record is surfaced under
// the store root (for doctor / status readers) instead of only a log line.
func TestRebuildWorker_DeadLetterIsObservable(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "deadletter-group"
	dir := requestsDirForGroup(group)
	payload, err := json.Marshal(proto.RebuildArgs{Group: group})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Pre-persist an already-exhausted attempt claim so the very next apply
	// dead-letters immediately (Attempts == maxRebuildAttempts).
	if _, err := requests.Write(dir, requests.Record{
		Kind:     requests.KindRebuild,
		Payload:  payload,
		Attempts: maxRebuildAttempts,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		t.Fatal("rebuildFn must not run for an already dead-lettered request")
		return nil, "", nil
	}

	// Backoff gate would otherwise defer the dead-letter apply; zero it so the
	// exhausted claim is processed on this drain.
	d := newRequestsDrainer(nil, rebuildFn, nil)
	d.rebuilds.backoff = func(int) time.Duration { return 0 }
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	d.rebuilds.waitIdle()

	// Request is dead-lettered (removed) ...
	recs, err := requests.ListPending(dir)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected the request to be dead-lettered (removed), still pending: %d", len(recs))
	}
	// ... and an observable record exists.
	dls, err := ReadDeadLetters(requestsRoot())
	if err != nil {
		t.Fatalf("ReadDeadLetters: %v", err)
	}
	if len(dls) != 1 {
		t.Fatalf("expected exactly one observable dead-letter record, got %d", len(dls))
	}
	if dls[0].Group != group {
		t.Fatalf("dead-letter record group = %q, want %q", dls[0].Group, group)
	}
	if dls[0].Kind != string(requests.KindRebuild) {
		t.Fatalf("dead-letter record kind = %q, want %q", dls[0].Kind, requests.KindRebuild)
	}
}

// Coalescing review finding #1: two same-group KindRebuild requests with
// DIFFERENT semantic payloads (divergent slug/wipe) must NOT be collapsed —
// keying coalescing on the group alone silently drops one caller's rebuild
// work while telling them it succeeded. Both distinct rebuilds must run.
func TestDrainRequestsOnce_DivergentPayloadRebuildsNotCoalesced(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "divergent-group"
	dir := requestsDirForGroup(group)

	// A: wipe rebuild of repo-x (older). B: plain rebuild of repo-y (newer).
	writeReb := func(args proto.RebuildArgs, created time.Time) {
		payload, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload, CreatedAt: created}); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	now := time.Now()
	writeReb(proto.RebuildArgs{Group: group, Slug: "repo-x", Wipe: true}, now.Add(-time.Second))
	writeReb(proto.RebuildArgs{Group: group, Slug: "repo-y"}, now)

	var mu sync.Mutex
	seen := map[string]bool{}
	rebuildFn := func(a proto.RebuildArgs) ([]string, string, error) {
		mu.Lock()
		seen[a.Slug] = true
		mu.Unlock()
		return []string{"/repo"}, "", nil
	}

	if err := drainRequestsOnce(requestsRoot(), nil, rebuildFn, nil); err != nil {
		t.Fatalf("drainRequestsOnce: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !seen["repo-x"] {
		t.Fatalf("repo-x wipe rebuild was silently dropped by group-only coalescing (seen=%v)", seen)
	}
	if !seen["repo-y"] {
		t.Fatalf("repo-y rebuild did not run (seen=%v)", seen)
	}
}

// Coalescing review finding #2: two same-payload rebuilds carrying DISTINCT
// wizard completion tokens must both keep RebuildRequestPending true until the
// SURVIVOR actually finishes — acking a coalesced-away duplicate early flips
// its token false while graph_fb_mtime hasn't advanced, so that wizard reports
// a spurious FAILURE.
func TestDrainOnce_CoalescedTokensFlipOnlyOnSurvivorCompletion(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "token-group"
	dir := requestsDirForGroup(group)

	// Identical semantic payload (group+slug+wipe+incremental), distinct tokens.
	writeReb := func(token string, created time.Time) {
		payload, err := json.Marshal(proto.RebuildArgs{Group: group, Slug: "x", Wipe: true, ProgressToken: token})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload, CreatedAt: created}); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	now := time.Now()
	writeReb("TA", now.Add(-time.Second)) // older
	writeReb("TB", now)                   // newer (survivor)

	block := make(chan struct{})
	started := make(chan struct{})
	var startOnce sync.Once
	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		startOnce.Do(func() { close(started) })
		<-block
		return []string{"/repo"}, "", nil
	}

	d := newRequestsDrainer(nil, rebuildFn, nil)
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(block)
		t.Fatal("survivor rebuild never started")
	}

	// While the survivor is IN-FLIGHT, BOTH tokens must still report pending.
	for _, tok := range []string{"TA", "TB"} {
		pending, err := RebuildRequestPending(group, tok)
		if err != nil {
			t.Fatalf("RebuildRequestPending(%s): %v", tok, err)
		}
		if !pending {
			t.Fatalf("token %s flipped to done while the survivor rebuild is still in-flight — spurious wizard failure", tok)
		}
	}

	close(block)
	d.rebuilds.waitIdle()

	// After real completion, BOTH tokens flip done.
	for _, tok := range []string{"TA", "TB"} {
		pending, err := RebuildRequestPending(group, tok)
		if err != nil {
			t.Fatalf("RebuildRequestPending(%s) post: %v", tok, err)
		}
		if pending {
			t.Fatalf("token %s still pending after the survivor completed", tok)
		}
	}
}

// Coalescing review finding #3: a fresh duplicate enqueue must NOT reset the
// crash-resume Attempts of an already-crash-looping pending record. Here a
// record that has already EXHAUSTED its attempt budget is joined by a fresh
// Attempts=0 duplicate of the identical payload; coalescing must carry the
// max attempt count forward so the survivor DEAD-LETTERS immediately rather
// than re-running the doomed rebuild from zero.
func TestDrainOnce_DuplicateDoesNotResetCrashAttempts(t *testing.T) {
	t.Setenv(SplitModeEnvVar, "1")
	root := t.TempDir()
	t.Setenv(EnvRoot, root)

	const group = "attempts-group"
	dir := requestsDirForGroup(group)

	payload, err := json.Marshal(proto.RebuildArgs{Group: group, Slug: "x"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	now := time.Now()
	// Crash-looping record that has already used its whole budget (older).
	if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload, Attempts: maxRebuildAttempts, CreatedAt: now.Add(-time.Second)}); err != nil {
		t.Fatalf("Write exhausted: %v", err)
	}
	// Fresh duplicate re-issue of the identical payload (newer, Attempts 0).
	if _, err := requests.Write(dir, requests.Record{Kind: requests.KindRebuild, Payload: payload, Attempts: 0, CreatedAt: now}); err != nil {
		t.Fatalf("Write duplicate: %v", err)
	}

	rebuildFn := func(proto.RebuildArgs) ([]string, string, error) {
		t.Error("rebuildFn ran — a duplicate enqueue reset the exhausted attempt budget instead of dead-lettering")
		return nil, "", nil
	}

	d := newRequestsDrainer(nil, rebuildFn, nil)
	d.rebuilds.backoff = func(int) time.Duration { return 0 }
	if err := d.drainOnce(requestsRoot()); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	d.rebuilds.waitIdle()

	recs, err := requests.ListPending(dir)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected both records gone (survivor dead-lettered, duplicate acked), still pending: %d", len(recs))
	}
	dls, err := ReadDeadLetters(requestsRoot())
	if err != nil {
		t.Fatalf("ReadDeadLetters: %v", err)
	}
	if len(dls) != 1 {
		t.Fatalf("expected exactly one dead-letter record, got %d", len(dls))
	}
}
