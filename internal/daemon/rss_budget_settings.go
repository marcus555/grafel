package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cajasmota/grafel/internal/process"
	"github.com/cajasmota/grafel/internal/registry"
)

const (
	MinConfiguredRSSBudgetMB = 100
	MaxConfiguredRSSBudgetMB = 32768
)

func settingsJSONPath() (string, error) {
	home, err := registry.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "settings.json"), nil
}

// ConfiguredRSSBudgetMB reads daemon_rss_budget_mb from settings.json.
// It returns 0 when no valid configured value exists.
func ConfiguredRSSBudgetMB() int64 {
	p, err := settingsJSONPath()
	if err != nil {
		return 0
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	var raw struct {
		DaemonRSSBudgetMB int64 `json:"daemon_rss_budget_mb"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0
	}
	if raw.DaemonRSSBudgetMB < MinConfiguredRSSBudgetMB || raw.DaemonRSSBudgetMB > MaxConfiguredRSSBudgetMB {
		return 0
	}
	return raw.DaemonRSSBudgetMB
}

// PersistConfiguredRSSBudgetMB writes daemon_rss_budget_mb into settings.json
// while preserving unrelated settings keys.
func PersistConfiguredRSSBudgetMB(mb int64) error {
	if mb < MinConfiguredRSSBudgetMB || mb > MaxConfiguredRSSBudgetMB {
		return fmt.Errorf("daemon_rss_budget_mb must be %d-%d; got %d",
			MinConfiguredRSSBudgetMB, MaxConfiguredRSSBudgetMB, mb)
	}
	p, err := settingsJSONPath()
	if err != nil {
		return err
	}
	settings := map[string]any{}
	if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("settings.json: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("settings.json: %w", err)
	}
	settings["daemon_rss_budget_mb"] = mb
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".settings.json.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := renameReplace(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// WorkstationRSSBudgetMB returns the admission budget preset used when switching
// to workstation mode. It scales with RAM and caps at 8 GB, matching the mode's
// intent without making background installs heavy by default.
func WorkstationRSSBudgetMB() int64 {
	total := process.TotalMemoryMB()
	if total <= 0 {
		return 2048
	}
	budget := total / 8
	if budget < 2048 {
		budget = 2048
	}
	if budget > 8192 {
		budget = 8192
	}
	return budget
}

func renameReplace(src, dst string) error {
	const (
		attempts  = 20
		pollEvery = 5 * time.Millisecond
	)
	var err error
	for i := 0; i < attempts; i++ {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		time.Sleep(pollEvery)
	}
	return err
}
