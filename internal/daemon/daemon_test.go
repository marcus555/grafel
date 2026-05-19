package daemon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cajasmota/archigraph/internal/daemon"
	"github.com/cajasmota/archigraph/internal/daemon/client"
	"github.com/cajasmota/archigraph/internal/daemon/proto"
)

// shortTempRoot returns a /tmp-rooted directory short enough for
// macOS's AF_UNIX sun_path limit (~103 bytes). t.TempDir() routes
// through TMPDIR which on macOS is well over the limit when combined
// with the daemon's "sockets/daemon.sock" suffix.
func shortTempRoot(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "archi-dt-")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// runDaemonForTest starts daemon.Run in a goroutine and returns the
// layout (for dialing) plus a stop function. The returned context is
// the one the daemon listens on — cancel it to force shutdown if Stop
// over RPC is what's being exercised.
func runDaemonForTest(t *testing.T, idx daemon.IndexFunc, rb daemon.RebuildFunc) daemon.Layout {
	t.Helper()
	root := shortTempRoot(t)
	t.Setenv(daemon.EnvRoot, root)
	layout, err := daemon.DefaultLayout()
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if err := daemon.EnsureLayout(layout); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Config{
			Layout:  layout,
			Index:   idx,
			Rebuild: rb,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Logf("daemon did not exit within 3s")
		}
	})
	// Wait for the socket to appear.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(layout.SocketPath); err == nil {
			return layout
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("daemon never bound socket at %s", layout.SocketPath)
	return layout
}

// TestDaemon_PingStatus verifies the most basic RPC round-trip and that
// Status reports a sensible RSS/pid/uptime. Anything reporting "RSS=0"
// would be a clear sign the daemon never warmed up memstats.
func TestDaemon_PingStatus(t *testing.T) {
	layout := runDaemonForTest(t, nil, nil)
	c, err := client.DialPath(layout.SocketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if r, err := c.Ping(); err != nil || r.Version == "" {
		t.Fatalf("ping: %v %+v", err, r)
	}
	st, err := c.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.PID == 0 || st.RSSBytes == 0 || st.SocketPath != layout.SocketPath {
		t.Fatalf("status looks wrong: %+v", st)
	}
}

// TestDaemon_IndexRPC stubs an IndexFunc and asserts the wire surface
// (arg shape, stats handoff) before any real extractor runs.
func TestDaemon_IndexRPC(t *testing.T) {
	idx := func(args proto.IndexArgs) (string, string, error) {
		if args.RepoPath == "" {
			return "", "", errors.New("empty repo")
		}
		return filepath.Join(args.RepoPath, ".archigraph", "graph.json"),
			`{"repo":"` + args.RepoPath + `","files":42}`, nil
	}
	layout := runDaemonForTest(t, idx, nil)
	c, err := client.DialPath(layout.SocketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	reply, err := c.Index(proto.IndexArgs{RepoPath: "/tmp/fake-repo"})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if reply.GraphPath != "/tmp/fake-repo/.archigraph/graph.json" {
		t.Fatalf("graph path: %q", reply.GraphPath)
	}
	if reply.StatsJSON == "" {
		t.Fatalf("stats empty")
	}
}

// TestDaemon_StopRPC closes the daemon over RPC and asserts that a
// subsequent dial reports ErrDaemonNotRunning, exactly as the CLI's
// thin clients depend on.
func TestDaemon_StopRPC(t *testing.T) {
	layout := runDaemonForTest(t, nil, nil)
	c, err := client.DialPath(layout.SocketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	_ = c.Close()
	// Give the listener a moment to close.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(layout.SocketPath); os.IsNotExist(err) {
			return // happy path: socket gone, daemon exited cleanly
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("socket %s still present after stop", layout.SocketPath)
}

// TestDaemon_ClientReportsNotRunning probes the canonical "no daemon"
// case: dialing a socket path that doesn't exist returns
// ErrDaemonNotRunning, not some opaque syscall error.
func TestDaemon_ClientReportsNotRunning(t *testing.T) {
	_, err := client.DialPath(filepath.Join(shortTempRoot(t), "nope.sock"))
	if !errors.Is(err, client.ErrDaemonNotRunning) {
		t.Fatalf("want ErrDaemonNotRunning, got %v", err)
	}
}
