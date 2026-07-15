//go:build windows

package statusfile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isRetryableSharingViolation reports whether err is (or wraps) Windows'
// ERROR_SHARING_VIOLATION — the transient error a concurrent os.Open can hit
// while Write's os.Rename is replacing the status file on NTFS. Read retries
// on this specific error only; any other error (including a genuine
// not-exist) is returned to the caller immediately.
func isRetryableSharingViolation(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
