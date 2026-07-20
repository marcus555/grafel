//go:build !windows

package statusfile_test

// isTransientReplaceErr always returns false on POSIX: os.Rename replaces the
// destination atomically even while a reader holds it open, so a concurrent
// Read never observes the "being used by another process" window Windows/NTFS
// can transiently produce. Because this is always false here, the concurrent
// test's transientUnavail counter stays 0 on macOS/Linux and the test behaves
// EXACTLY like the strict zero-tolerance version — only the Windows-inherent
// transient is ever tolerated.
func isTransientReplaceErr(err error) bool {
	return false
}
