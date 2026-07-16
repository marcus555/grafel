package statusfile

import (
	"errors"
	"testing"
	"time"
)

// TestRenameWithRetry_SucceedsAfterTransientFailures is the darwin-safe unit
// test of the bounded retry Write performs on a transient Windows
// replace-open-file error (#5729-W1 follow-up: os.Rename over a file a
// concurrent reader holds open can fail with ERROR_ACCESS_DENIED on NTFS, and
// Write had no retry). It cannot produce the real Windows syscall error on this
// platform, so it injects a sentinel through renameFn and monkey-patches the
// classifier + backoff sleep for the duration of the test — proving
// renameWithRetry's control flow (retry until the transient error clears, then
// succeed) independent of the platform-specific classifier.
func TestRenameWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	sentinel := errors.New("simulated ERROR_ACCESS_DENIED")

	origRename := renameFn
	origClassifier := isRetryableReplaceErrorFn
	origSleep := renameRetrySleep
	t.Cleanup(func() {
		renameFn = origRename
		isRetryableReplaceErrorFn = origClassifier
		renameRetrySleep = origSleep
	})
	isRetryableReplaceErrorFn = func(err error) bool { return errors.Is(err, sentinel) }
	renameRetrySleep = func(time.Duration) {} // no-op backoff so the test does not sleep

	attempts := 0
	const failUntilAttempt = 3 // succeeds on the 3rd call, within renameRetryAttempts
	renameFn = func(from, to string) error {
		attempts++
		if attempts < failUntilAttempt {
			return sentinel
		}
		return nil
	}

	if err := renameWithRetry("/fake/from", "/fake/to"); err != nil {
		t.Fatalf("renameWithRetry: unexpected error after transient failures: %v", err)
	}
	if attempts != failUntilAttempt {
		t.Errorf("attempts = %d, want %d (N transient failures + 1 success)", attempts, failUntilAttempt)
	}
}

// TestRenameWithRetry_NonRetryableErrorReturnsImmediately asserts a
// non-replace-open error (e.g. a genuine cross-device / permission error that
// will never clear) is NOT retried — only the specific transient error is
// worth the backoff.
func TestRenameWithRetry_NonRetryableErrorReturnsImmediately(t *testing.T) {
	other := errors.New("some other rename error")

	origRename := renameFn
	origSleep := renameRetrySleep
	t.Cleanup(func() {
		renameFn = origRename
		renameRetrySleep = origSleep
	})
	renameRetrySleep = func(time.Duration) {}

	attempts := 0
	renameFn = func(from, to string) error {
		attempts++
		return other
	}

	err := renameWithRetry("/fake/from", "/fake/to")
	if !errors.Is(err, other) {
		t.Fatalf("renameWithRetry: err = %v, want other", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (non-retryable error must not be retried)", attempts)
	}
}

// TestRenameWithRetry_ExhaustsAndReturnsLastError asserts a replace-open error
// that never clears is retried exactly renameRetryAttempts times and then
// surfaced to the caller — Write must not retry forever nor swallow a genuinely
// persistent failure (so a writer can never hang indefinitely).
func TestRenameWithRetry_ExhaustsAndReturnsLastError(t *testing.T) {
	sentinel := errors.New("simulated persistent ERROR_ACCESS_DENIED")

	origRename := renameFn
	origClassifier := isRetryableReplaceErrorFn
	origSleep := renameRetrySleep
	t.Cleanup(func() {
		renameFn = origRename
		isRetryableReplaceErrorFn = origClassifier
		renameRetrySleep = origSleep
	})
	isRetryableReplaceErrorFn = func(err error) bool { return errors.Is(err, sentinel) }
	renameRetrySleep = func(time.Duration) {}

	attempts := 0
	renameFn = func(from, to string) error {
		attempts++
		return sentinel
	}

	err := renameWithRetry("/fake/from", "/fake/to")
	if !errors.Is(err, sentinel) {
		t.Fatalf("renameWithRetry: err = %v, want sentinel after exhausting retries", err)
	}
	if attempts != renameRetryAttempts {
		t.Errorf("attempts = %d, want exactly renameRetryAttempts=%d", attempts, renameRetryAttempts)
	}
	// The write-side budget is intentionally DECOUPLED from (and larger than)
	// the read-side budget: the rename must outlast a concurrent reader's
	// reopen loop, whereas a read is a single ReadFile. Lock that in so the two
	// can't silently be re-coupled.
	if renameRetryAttempts <= readRetryAttempts {
		t.Errorf("renameRetryAttempts=%d must exceed readRetryAttempts=%d (write path must be more patient)",
			renameRetryAttempts, readRetryAttempts)
	}
}
