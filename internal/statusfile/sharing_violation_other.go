//go:build !windows

package statusfile

// isRetryableReplaceError always returns false on POSIX platforms: Write's
// tmp+rename is atomic at the filesystem level there — os.Rename replaces the
// destination even while a reader holds it open, and a concurrent Read never
// observes a torn/sharing-conflicted open the way Windows/NTFS can transiently
// produce during os.Rename's replace. Keeping this a no-op on POSIX preserves
// both Read's and Write's existing single-attempt behavior exactly (zero
// behavior change on POSIX).
func isRetryableReplaceError(err error) bool {
	return false
}
