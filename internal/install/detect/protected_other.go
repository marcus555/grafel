// protected_other.go — non-macOS stub for the TCC-protected-folder guard. Off
// darwin there is no TCC, so the sibling-repo scan runs unchanged.

//go:build !darwin

package detect

func isProtectedScanParent(parent string) bool { return false }

func isProtectedHomeChild(parent, name string) bool { return false }
