package statusfile_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/statusfile"
)

// TestWrite_ConcurrentSameRepo_NoTornRead reproduces the review #5734 BLOCKING
// finding: concurrent Writes to the SAME repo must never publish a torn/garbled
// file that a concurrent Read decodes as corrupt.
//
// With the original fixed tmp name (`tmp := path + ".tmp"`), N writers all
// O_TRUNC the same tmp inode and interleave, then rename a garbled file into
// place — a looping reader intermittently gets a JSON unmarshal error and (in
// the CLI) falls back to Status:"unknown" mid-index, exactly what the status
// plane must not do. The reviewer measured 2 corrupt reads / 1884.
//
// THE INVARIANT (#5788): "Write never publishes torn/partial content." That is
// asserted at exactly ZERO via tornContent below. It is NOT "the file is always
// openable" — that is impossible on Windows/NTFS: a legitimate atomic replace
// goes through MoveFileEx, which briefly holds the destination EXCLUSIVELY, so
// a concurrent os.Open (or a competing os.Rename) can transiently see
// ERROR_SHARING_VIOLATION / ERROR_ACCESS_DENIED ("being used by another
// process"). That is "temporarily unavailable during a legitimate replace,"
// NOT corruption. The read/write paths retry these over a bounded budget
// (readRetryAttempts / renameRetryAttempts), but under this test's ADVERSARIAL
// 8-writer over-subscription the target is under near-constant rename churn, so
// a small residual fraction can outlast even the bounded retry.
//
// Production has a SINGLE serialized writer (the daemon statusWriter goroutine),
// so this transient window essentially never occurs there; this test
// deliberately over-subscribes writers to stress atomicity, so a small bounded
// transient-unavailable rate is EXPECTED and is categorically distinct from
// corruption. We therefore split the old single `corrupt` counter (which
// conflated the two and mis-counted the Windows transient as corruption) into:
//
//   - tornContent      — MUST be 0: a read that returned complete-but-wrong
//     content, a decode/partial-content error, or ANY read/write error that is
//     NOT the transient-replace class. This is the real atomicity guarantee.
//   - transientUnavail — allowed but BOUNDED: a read/write that failed with the
//     Windows transient-replace class AFTER its bounded retry was exhausted.
//
// On POSIX isTransientReplaceErr is always false, so transientUnavail stays 0
// and this behaves EXACTLY like the strict zero-tolerance version.
func TestWrite_ConcurrentSameRepo_NoTornRead(t *testing.T) {
	t.Setenv("GRAFEL_HOME", t.TempDir())

	repo := "/some/repo/under/concurrent/write"

	// Seed once so the reader always finds the file (a legitimate NotExist
	// before the first write is not a torn read).
	if err := statusfile.Write(repo, &statusfile.File{EnginePID: 1, RepoPath: repo}); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	const (
		writers         = 8
		writesPerWorker = 400
	)

	var (
		writersWG        sync.WaitGroup
		readerWG         sync.WaitGroup
		stop             atomic.Bool
		tornContent      atomic.Int64 // hard invariant: MUST be 0
		transientUnavail atomic.Int64 // Windows-inherent transient replace window; bounded
		reads            atomic.Int64
		firstTorn        atomic.Value // string: first torn-content description
	)

	// Reader: spins reading the file, classifying every failure. os.IsNotExist
	// is impossible after the seed write (Write renames atomically), so it never
	// counts. A transient-replace error (Windows only) is "temporarily
	// unavailable," not corruption. Anything else — a decode error, or a
	// complete-but-wrong RepoPath — is torn content.
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for !stop.Load() {
			f, err := statusfile.Read(repo)
			reads.Add(1)
			if err != nil {
				switch {
				case os.IsNotExist(err):
					// impossible post-seed; never a torn read.
				case isTransientReplaceErr(err):
					// Legitimate atomic-replace window outlasted the bounded
					// retry — temporarily unavailable, NOT corrupt.
					transientUnavail.Add(1)
				default:
					// A JSON decode / partial-content error, or any other hard
					// failure: the reader saw garbage.
					tornContent.Add(1)
					firstTorn.CompareAndSwap(nil, "read error: "+err.Error())
				}
				continue
			}
			// A well-formed-but-truncated JSON could still decode to a
			// zero-value struct, so assert an invariant every writer upholds.
			// A wrong RepoPath is complete-but-wrong content — torn, never
			// transient.
			if f.RepoPath != repo {
				tornContent.Add(1)
				firstTorn.CompareAndSwap(nil, "unexpected RepoPath: "+f.RepoPath)
			}
		}
	}()

	for w := 0; w < writers; w++ {
		writersWG.Add(1)
		go func(id int) {
			defer writersWG.Done()
			for i := 0; i < writesPerWorker; i++ {
				f := &statusfile.File{
					EnginePID:     id*writesPerWorker + i,
					HeartbeatAt:   time.Now().UTC(),
					Version:       "concurrent-writer",
					RepoPath:      repo,
					IndexedCommit: "deadbeefcafe",
					Entities:      int64(i),
					Relationships: int64(id),
				}
				if err := statusfile.Write(repo, f); err != nil {
					// Same split as the read side: a transient-replace failure
					// after Write's bounded rename retry is over-subscription
					// churn, not a broken write; any other write error is a hard
					// failure.
					if isTransientReplaceErr(err) {
						transientUnavail.Add(1)
						continue
					}
					tornContent.Add(1)
					firstTorn.CompareAndSwap(nil, "write error: "+err.Error())
					return
				}
			}
		}(w)
	}

	writersWG.Wait()
	stop.Store(true)
	readerWG.Wait()

	// Hard invariant: Write must NEVER publish torn/partial/wrong content.
	if tc := tornContent.Load(); tc != 0 {
		t.Fatalf("torn/corrupt reads = %d over %d reads (first: %v) — concurrent same-repo Write must be atomic",
			tc, reads.Load(), firstTorn.Load())
	}
	if reads.Load() == 0 {
		t.Fatal("reader never observed a read — test did not exercise the race")
	}
	// Bounded transient tolerance: the Windows atomic-replace window is real but
	// rare (CI observed ~0.4-0.6%). Require >95% of reads to succeed so a REAL
	// regression that makes reads/writes fail constantly still trips, while the
	// inherent transient rate passes with wide margin. On POSIX transientUnavail
	// is always 0, so this is a strict zero-tolerance test there.
	total := reads.Load()
	if tu := transientUnavail.Load(); tu*100 >= total*5 {
		t.Fatalf("transient-unavailable reads/writes = %d over %d reads (>=5%%) — retry budget or atomicity regressed",
			tu, total)
	}
}
