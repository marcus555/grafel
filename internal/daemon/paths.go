// Package daemon implements the long-running archigraph process per
// ADR-0017. It exposes a JSON-RPC service over a Unix-domain socket and
// owns indexing, cross-repo linking, and (in later phases) fsnotify
// watchers and the MCP query surface.
package daemon

import (
	"os"
	"path/filepath"
)

// Layout is the on-disk footprint of a daemon: socket path, pid file,
// log directory. Everything lives under ~/.archigraph/ so the daemon
// shares state with the existing registry and group configs.
type Layout struct {
	Root       string // ~/.archigraph
	SocketDir  string // ~/.archigraph/sockets
	SocketPath string // ~/.archigraph/sockets/daemon.sock
	PIDPath    string // ~/.archigraph/daemon.pid
	LogDir     string // ~/.archigraph/logs
	LogPath    string // ~/.archigraph/logs/daemon.log
}

// EnvRoot is honoured by DefaultLayout when set; lets tests point a
// freshly-built daemon at a tempdir without touching the real ~/.archigraph.
const EnvRoot = "ARCHIGRAPH_DAEMON_ROOT"

// DefaultLayout returns the standard on-disk layout for the current
// user. It does not create directories; callers responsible for daemon
// startup invoke EnsureLayout. We split the two so client-side code
// (which only reads the socket path) does not have to be a writer.
//
// When ARCHIGRAPH_DAEMON_ROOT is set, that directory replaces ~/.archigraph
// for all paths in the returned layout. This is used exclusively by tests.
func DefaultLayout() (Layout, error) {
	root := os.Getenv(EnvRoot)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Layout{}, err
		}
		root = filepath.Join(home, ".archigraph")
	}
	socketDir := filepath.Join(root, "sockets")
	logDir := filepath.Join(root, "logs")
	return Layout{
		Root:       root,
		SocketDir:  socketDir,
		SocketPath: filepath.Join(socketDir, "daemon.sock"),
		PIDPath:    filepath.Join(root, "daemon.pid"),
		LogDir:     logDir,
		LogPath:    filepath.Join(logDir, "daemon.log"),
	}, nil
}

// EnsureLayout creates the directories the daemon writes to. The socket
// path itself is not created here — the listener does that. Permissions
// are 0700/0600 because the daemon shares state across nothing and the
// socket is per-user.
func EnsureLayout(l Layout) error {
	for _, d := range []string{l.Root, l.SocketDir, l.LogDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}
