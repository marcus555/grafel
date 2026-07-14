package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguredMemorySettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	p := filepath.Join(home, "settings.json")
	if err := os.WriteFile(p, []byte(`{
  "daemon_rss_budget_mb": 8192,
  "daemon_go_memory_limit_mb": 6144,
  "unrelated": true
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ConfiguredRSSBudgetMB(); got != 8192 {
		t.Fatalf("RSS budget = %d, want 8192", got)
	}
	if got := ConfiguredGoMemoryLimitMB(); got != 6144 {
		t.Fatalf("Go memory limit = %d, want 6144", got)
	}
}

func TestConfiguredGoMemoryLimitMB_AbsentOrInvalidUsesAuto(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	p := filepath.Join(home, "settings.json")

	for _, body := range []string{
		`{"daemon_rss_budget_mb":8192}`,
		`{"daemon_go_memory_limit_mb":99}`,
		`{"daemon_go_memory_limit_mb":32769}`,
		`not-json`,
	} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := ConfiguredGoMemoryLimitMB(); got != 0 {
			t.Fatalf("ConfiguredGoMemoryLimitMB(%q) = %d, want 0", body, got)
		}
	}
}
