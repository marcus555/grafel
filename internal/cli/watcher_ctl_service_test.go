package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon"
)

// These tests exercise the launchd-detection gate added for issue #5789:
// `grafel start`/`grafel restart` must route through the OS-service-aware
// restart (serviceRestartForThisRoot) instead of forking a manual,
// service-manager-blind daemon (manualForkStart) when an OS service
// (launchd/systemd/schtasks) is already registered for this root. Forking a
// manual daemon in that case races the service manager's own respawn over
// the pidfile/socket, and the pidfile's wedged-daemon reclaim can then
// SIGKILL one of the two mid-startup.
//
// The seams (serviceInstalledForThisRoot / serviceRestartForThisRoot /
// manualForkStart) are function vars so these tests can spy on which path
// was taken without touching a real launchctl/systemctl/schtasks or forking
// a real process.

// stubServiceSeams overrides the three seams for the duration of the test
// and restores them on cleanup. It returns pointers to call counters.
func stubServiceSeams(t *testing.T, installed bool) (restartCalls, forkCalls *int) {
	t.Helper()
	restartCalls = new(int)
	forkCalls = new(int)

	origInstalled := serviceInstalledForThisRoot
	origRestart := serviceRestartForThisRoot
	origFork := manualForkStart
	t.Cleanup(func() {
		serviceInstalledForThisRoot = origInstalled
		serviceRestartForThisRoot = origRestart
		manualForkStart = origFork
	})

	serviceInstalledForThisRoot = func() bool { return installed }
	serviceRestartForThisRoot = func(_ io.Writer) error {
		*restartCalls++
		return nil
	}
	manualForkStart = func(_ io.Writer, _ daemon.Layout, _ int64, _ bool) error {
		*forkCalls++
		return nil
	}
	return restartCalls, forkCalls
}

func TestRunDaemonStartOpts_RoutesToServiceRestart_WhenInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(daemon.EnvRoot, dir)

	restartCalls, forkCalls := stubServiceSeams(t, true)

	var out bytes.Buffer
	if err := runDaemonStartOpts(&out, 0, false); err != nil {
		t.Fatalf("runDaemonStartOpts: %v", err)
	}
	if *restartCalls != 1 {
		t.Fatalf("expected serviceRestartForThisRoot to be called once, got %d", *restartCalls)
	}
	if *forkCalls != 0 {
		t.Fatalf("expected manualForkStart NOT to be called, got %d calls", *forkCalls)
	}
}

func TestRunDaemonStartOpts_FallsBackToManualFork_WhenNotInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(daemon.EnvRoot, dir)

	restartCalls, forkCalls := stubServiceSeams(t, false)

	var out bytes.Buffer
	if err := runDaemonStartOpts(&out, 0, false); err != nil {
		t.Fatalf("runDaemonStartOpts: %v", err)
	}
	if *forkCalls != 1 {
		t.Fatalf("expected manualForkStart to be called once, got %d", *forkCalls)
	}
	if *restartCalls != 0 {
		t.Fatalf("expected serviceRestartForThisRoot NOT to be called, got %d calls", *restartCalls)
	}
}

func TestRunDaemonRestart_RoutesToServiceRestart_WhenInstalled_NoManualForkOrPidfileClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(daemon.EnvRoot, dir)

	restartCalls, forkCalls := stubServiceSeams(t, true)

	var out bytes.Buffer
	if err := runDaemonRestart(&out); err != nil {
		t.Fatalf("runDaemonRestart: %v", err)
	}
	if *restartCalls != 1 {
		t.Fatalf("expected serviceRestartForThisRoot to be called once, got %d", *restartCalls)
	}
	if *forkCalls != 0 {
		t.Fatalf("expected manualForkStart NOT to be called (would race the pidfile), got %d calls", *forkCalls)
	}

	// The launchd-safe restart must NOT have gone through the blind
	// stop→wait→clear-pidfile sequence: no pidfile should have been written
	// by this call (service.Restart owns the stop/start dance internally, so
	// runDaemonRestart's own stop/wait/clear-pidfile machinery must be
	// bypassed entirely when routing here).
	layout, err := daemon.DefaultLayout()
	if err != nil {
		t.Fatalf("daemon.DefaultLayout: %v", err)
	}
	if _, statErr := os.Stat(layout.PIDPath); statErr == nil {
		t.Fatalf("pidfile should not exist — the service-restart path must not touch it")
	}
}

func TestRunDaemonRestart_FallsBackToManualPath_WhenNotInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(daemon.EnvRoot, dir)

	restartCalls, forkCalls := stubServiceSeams(t, false)

	var out bytes.Buffer
	if err := runDaemonRestart(&out); err != nil {
		t.Fatalf("runDaemonRestart: %v", err)
	}
	if *forkCalls != 1 {
		t.Fatalf("expected the manual start path to run once, got %d", *forkCalls)
	}
	if *restartCalls != 0 {
		t.Fatalf("expected serviceRestartForThisRoot NOT to be called, got %d calls", *restartCalls)
	}
}
