package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Windows counterpart: on Windows service.RegisteredRoot() is a stub that
// always returns found==false, so blocking defect #2 was that the detector's
// old `!found → return false` early-out fired BEFORE Status() was consulted,
// making the gate unreachable for a real schtasks service. The fix lets the
// detector fall through to service.Status(), which stats the task XML. This
// test locks that: with the task XML present, the detector must report
// installed. (Runs in Windows CI; compiled-checked elsewhere via GOOS=windows.)

func TestDefaultServiceInstalledForThisRoot_Windows_TaskPresent(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)

	// Mirror service.taskXMLPath(): %LOCALAPPDATA%\grafel\tasks\com.grafel.daemon.xml
	xmlPath := filepath.Join(local, "grafel", "tasks", "com.grafel.daemon.xml")
	if err := os.MkdirAll(filepath.Dir(xmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xmlPath, []byte("<Task></Task>"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report INSTALLED when the schtasks task XML exists " +
			"(defect #2: the old !found early-out made this unreachable on Windows)")
	}
}

func TestDefaultServiceInstalledForThisRoot_Windows_NoTask(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	if defaultServiceInstalledForThisRoot() {
		t.Fatal("detector must report NOT-installed when no task XML exists")
	}
}
