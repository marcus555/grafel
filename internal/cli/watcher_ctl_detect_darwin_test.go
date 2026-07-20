package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/service"
)

// These tests exercise the REAL detector defaultServiceInstalledForThisRoot
// against a REAL launchd plist rendered by service.GeneratePlist — the code
// path where issue #5789's blocking defect #1 lived. The seam-stubbing tests
// in watcher_ctl_service_test.go replace the detector wholesale and so never
// touch this logic; that gap is exactly why the HOME-vs-~/.grafel comparison
// bug shipped. The MatchingHome case below goes RED against the pre-fix code
// (which compared the baked HOME against layout.Root = HOME/.grafel and thus
// always mismatched → always reported NOT-installed → race reintroduced).

// darwinPlistRelPath is the plist location relative to HOME that the darwin
// status()/registeredRoot() read (mirrors service.plistPath()).
const darwinPlistRelPath = "Library/LaunchAgents/com.grafel.daemon.plist"

// writeDarwinPlist renders a real plist whose baked <key>HOME</key> is
// bakedHome, and writes it to fileHome's LaunchAgents path.
func writeDarwinPlist(t *testing.T, fileHome, bakedHome string) {
	t.Helper()
	// GeneratePlist bakes HOME from os.UserHomeDir(); swap HOME to bakedHome
	// for the render only, then restore it (the outer t.Setenv cleanup will
	// restore the pre-test value regardless).
	prev := os.Getenv("HOME")
	if err := os.Setenv("HOME", bakedHome); err != nil {
		t.Fatal(err)
	}
	plist, err := service.GeneratePlist(service.Options{
		BinPath: "/opt/grafel/bin/grafel",
		LogDir:  filepath.Join(bakedHome, ".grafel", "logs"),
	})
	if err != nil {
		t.Fatalf("GeneratePlist: %v", err)
	}
	if err := os.Setenv("HOME", prev); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(fileHome, darwinPlistRelPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, plist, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultServiceInstalledForThisRoot_Darwin_MatchingHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Real plist installed for THIS HOME.
	writeDarwinPlist(t, home, home)

	if !defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report INSTALLED for a plist baked with THIS HOME " +
			"(defect #1: comparing baked HOME against ~/.grafel made this always false, " +
			"reintroducing the #5789 manual-fork race for every real install)")
	}
}

func TestDefaultServiceInstalledForThisRoot_Darwin_DifferentHome(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	t.Setenv("HOME", home)

	// Plist file lives under our HOME but is baked for a DIFFERENT user's HOME
	// (e.g. a global-label service owned by another root). We must not claim it.
	writeDarwinPlist(t, home, other)

	if defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report NOT-ours when the plist's baked HOME differs from this HOME")
	}
}

func TestDefaultServiceInstalledForThisRoot_Darwin_NoService(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No plist written — nothing installed.
	if defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report NOT-installed when no plist exists")
	}
}
