//go:build !windows

package statusfile

// isRetryableSharingViolation always returns false on POSIX platforms:
// Write's tmp+rename is atomic at the filesystem level there, so a
// concurrent Read never observes a torn/sharing-conflicted open the way
// Windows/NTFS can transiently produce during os.Rename's replace. Keeping
// this a no-op on POSIX preserves Read's existing single-attempt behavior
// exactly.
func isRetryableSharingViolation(err error) bool {
	return false
}
