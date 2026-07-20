package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/service"
)

// Linux counterpart of the darwin real-detector tests: exercise
// defaultServiceInstalledForThisRoot against a REAL systemd --user unit
// rendered by service.GenerateUnit. Same #5789 defect #1 coverage — the baked
// Environment=HOME= must be compared on the HOME dimension, not ~/.grafel.

// linuxUnitRelPath is the unit location relative to HOME that the linux
// status()/registeredRoot() read (mirrors service.unitPath()).
const linuxUnitRelPath = ".config/systemd/user/grafel-daemon.service"

func writeLinuxUnit(t *testing.T, fileHome, bakedHome string) {
	t.Helper()
	prev := os.Getenv("HOME")
	if err := os.Setenv("HOME", bakedHome); err != nil {
		t.Fatal(err)
	}
	unit, err := service.GenerateUnit(service.Options{
		BinPath: "/opt/grafel/bin/grafel",
		LogDir:  filepath.Join(bakedHome, ".grafel", "logs"),
	})
	if err != nil {
		t.Fatalf("GenerateUnit: %v", err)
	}
	if err := os.Setenv("HOME", prev); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(fileHome, linuxUnitRelPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, unit, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultServiceInstalledForThisRoot_Linux_MatchingHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeLinuxUnit(t, home, home)

	if !defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report INSTALLED for a unit baked with THIS HOME " +
			"(defect #1: comparing baked HOME against ~/.grafel made this always false)")
	}
}

func TestDefaultServiceInstalledForThisRoot_Linux_DifferentHome(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	t.Setenv("HOME", home)

	writeLinuxUnit(t, home, other)

	if defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report NOT-ours when the unit's baked HOME differs from this HOME")
	}
}

func TestDefaultServiceInstalledForThisRoot_Linux_NoService(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report NOT-installed when no unit exists")
	}
}
