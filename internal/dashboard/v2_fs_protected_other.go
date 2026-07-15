// v2_fs_protected_other.go — non-macOS stubs for the TCC-protected-folder
// guard. Off darwin there is no TCC, so nothing is "protected": both predicates
// are false and the folder browser behaves exactly as before.

//go:build !darwin

package dashboard

func protectedHomeChild(home, parent, name string) bool { return false }

func protectedProbePath(home, path string) bool { return false }
