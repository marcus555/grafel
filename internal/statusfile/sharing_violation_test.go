package statusfile

import (
	"errors"
	"testing"
)

// TestReadFileWithRetry_RetriesOnSharingViolation is a darwin-safe unit test
// of the bounded retry Read performs on a transient Windows sharing
// violation (#5729-W1 follow-up: a concurrent os.Open during Write's
// tmp+rename can hit ERROR_SHARING_VIOLATION on NTFS, and Read had no
// retry). It cannot exercise the real Windows syscall error on this
// platform, so it fakes isRetryableSharingViolation's "yes, retry" branch by
// injecting a sentinel error through readFile and monkey-patching the
// classifier for the duration of the test — proving readFileWithRetry's
// control flow (retry N times, then return the last error) independent of
// the platform-specific classifier.
func TestReadFileWithRetry_RetriesOnSharingViolation(t *testing.T) {
	sentinel := errors.New("simulated ERROR_SHARING_VIOLATION")

	origReadFile := readFile
	origClassifier := isRetryableSharingViolationFn
	t.Cleanup(func() {
		readFile = origReadFile
		isRetryableSharingViolationFn = origClassifier
	})
	isRetryableSharingViolationFn = func(err error) bool {
		return errors.Is(err, sentinel)
	}

	attempts := 0
	const failUntilAttempt = 3 // succeeds on the 3rd call, well within readRetryAttempts
	readFile = func(path string) ([]byte, error) {
		attempts++
		if attempts < failUntilAttempt {
			return nil, sentinel
		}
		return []byte(`{"engine_pid":1}`), nil
	}

	data, err := readFileWithRetry("/fake/path")
	if err != nil {
		t.Fatalf("readFileWithRetry: unexpected error after transient failures: %v", err)
	}
	if string(data) != `{"engine_pid":1}` {
		t.Errorf("readFileWithRetry: data = %q, want the eventual successful read", data)
	}
	if attempts != failUntilAttempt {
		t.Errorf("attempts = %d, want %d (should stop retrying once readFile succeeds)", attempts, failUntilAttempt)
	}
}

// TestReadFileWithRetry_ExhaustsAndReturnsLastError asserts a sharing
// violation that never clears is retried exactly readRetryAttempts times
// and then surfaced to the caller — Read must not retry forever nor swallow
// a genuinely persistent failure.
func TestReadFileWithRetry_ExhaustsAndReturnsLastError(t *testing.T) {
	sentinel := errors.New("simulated persistent ERROR_SHARING_VIOLATION")

	origReadFile := readFile
	origClassifier := isRetryableSharingViolationFn
	t.Cleanup(func() {
		readFile = origReadFile
		isRetryableSharingViolationFn = origClassifier
	})
	isRetryableSharingViolationFn = func(err error) bool {
		return errors.Is(err, sentinel)
	}

	attempts := 0
	readFile = func(path string) ([]byte, error) {
		attempts++
		return nil, sentinel
	}

	_, err := readFileWithRetry("/fake/path")
	if !errors.Is(err, sentinel) {
		t.Fatalf("readFileWithRetry: err = %v, want sentinel after exhausting retries", err)
	}
	if attempts != readRetryAttempts {
		t.Errorf("attempts = %d, want exactly readRetryAttempts=%d", attempts, readRetryAttempts)
	}
}

// TestReadFileWithRetry_NonRetryableErrorReturnsImmediately asserts a
// non-sharing-violation error (e.g. a genuine not-exist) is NOT retried —
// only the specific transient error is worth the backoff.
func TestReadFileWithRetry_NonRetryableErrorReturnsImmediately(t *testing.T) {
	other := errors.New("some other I/O error")

	origReadFile := readFile
	t.Cleanup(func() { readFile = origReadFile })

	attempts := 0
	readFile = func(path string) ([]byte, error) {
		attempts++
		return nil, other
	}

	_, err := readFileWithRetry("/fake/path")
	if !errors.Is(err, other) {
		t.Fatalf("readFileWithRetry: err = %v, want other", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (non-retryable error must not be retried)", attempts)
	}
}
