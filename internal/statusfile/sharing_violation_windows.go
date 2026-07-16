//go:build windows

package statusfile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isRetryableReplaceError reports whether err is (or wraps) one of the two
// transient NTFS errors that surface while Write's os.Rename is replacing the
// status file that a concurrent reader is holding open:
//
//   - ERROR_SHARING_VIOLATION — a concurrent os.Open racing the rename's
//     replace (the error the Read side has always retried on).
//   - ERROR_ACCESS_DENIED — os.Rename itself failing to replace the
//     destination while a reader still holds it open ("Access is denied").
//     This is the write-side mirror of the sharing violation and is what the
//     8-writer/1-reader concurrency test (TestWrite_ConcurrentSameRepo_NoTornRead)
//     hit on windows-latest.
//
// Both are momentary "replace an open file" conditions that clear once the
// other handle is released, so a short bounded retry rides them out. Any other
// error (including a genuine not-exist or a cross-device rename) is returned to
// the caller immediately.
func isRetryableReplaceError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
