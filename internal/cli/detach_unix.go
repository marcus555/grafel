//go:build darwin || linux

package cli

import "syscall"

// detachSysProcAttr returns a SysProcAttr that puts the spawned daemon
// in its own process group. Without Setsid the daemon would inherit
// the CLI's controlling terminal and die when the CLI exits; with it,
// the daemon is a session leader and survives.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
