//go:build windows

package statusfile_test

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransientReplaceErr reports whether err is (or wraps) one of the two
// transient NTFS "the destination is momentarily busy being atomically
// replaced" errors a concurrent reader's os.Open (or a writer's os.Rename) can
// hit while another writer's MoveFileEx briefly holds the destination
// exclusively: ERROR_SHARING_VIOLATION ("the process cannot access the file
// because it is being used by another process") and ERROR_ACCESS_DENIED. These
// mean "temporarily unavailable during a LEGITIMATE atomic replace" — NOT torn
// or corrupt content. It mirrors the production classifier
// statusfile.isRetryableReplaceError, duplicated here because that symbol is
// unexported and this test lives in the external statusfile_test package.
func isTransientReplaceErr(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
