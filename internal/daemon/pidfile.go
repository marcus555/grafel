package daemon

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ErrAlreadyRunning is returned by AcquirePIDFile when another daemon
// process holds the pid file. The pid of the live owner is included in
// the wrapped error message so callers can surface it directly.
var ErrAlreadyRunning = errors.New("daemon already running")

// AcquirePIDFile writes the current pid to path, returning a release
// closure. If path already contains a pid for a running process, the
// call returns ErrAlreadyRunning. Stale pid files (the named process is
// gone) are overwritten silently — a crash should never wedge startup.
//
// We deliberately do NOT use flock here: the goal is to detect another
// daemon, and pid+syscall.Kill(pid,0) is portable across darwin/linux
// without a new dependency.
func AcquirePIDFile(path string) (release func(), err error) {
	if existing, ok := readPID(path); ok && pidAlive(existing) {
		return nil, fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, existing)
	}
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write pid file %s: %w", path, err)
	}
	return func() {
		// Best-effort cleanup. Errors here are not actionable — if we
		// can't remove our own pid file, the next startup will see a
		// stale entry and overwrite it.
		_ = os.Remove(path)
	}, nil
}

// ReadPIDFile returns the pid recorded in path, or 0 if the file is
// missing/empty/unreadable. Clients use this for `archigraph status`.
func ReadPIDFile(path string) int {
	pid, _ := readPID(path)
	return pid
}

func readPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// pidAlive returns true when a process with the given pid exists and
// the current user can signal it. signal 0 is the POSIX existence
// probe; on darwin and linux it does not deliver a signal but does
// validate the pid.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil
}
