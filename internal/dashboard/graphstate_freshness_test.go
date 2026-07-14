package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/graph/fbwriter"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/testsupport"
)

func TestGraphCacheTTLRevalidatesUnchangedSourcesWithoutReload(t *testing.T) {
	group, graphPath := writeFreshnessTestGroup(t)
	cache := NewGraphCache(time.Millisecond)
	first, err := cache.GetGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDashGroupReaders(first) })
	time.Sleep(5 * time.Millisecond)

	second, err := cache.GetGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("unchanged sources were materialised again after TTL expiry")
	}
	if _, err := os.Stat(graphPath); err != nil {
		t.Fatal(err)
	}
}

func TestGraphCacheTTLReloadsChangedSources(t *testing.T) {
	group, graphPath := writeFreshnessTestGroup(t)
	cache := NewGraphCache(time.Millisecond)
	first, err := cache.GetGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	changedAt := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(graphPath, changedAt, changedAt); err != nil {
		t.Fatal(err)
	}

	second, err := cache.GetGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("changed graph artifact did not trigger a reload")
	}
	closeDashGroupReaders(first)
	closeDashGroupReaders(second)
}

func writeFreshnessTestGroup(t *testing.T) (string, string) {
	t.Helper()
	root := testsupport.IsolateHome(t)
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := daemon.StateDirForRepoRef(repoPath, "")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	graphPath := filepath.Join(stateDir, "graph.fb")
	doc := &graph.Document{Entities: []graph.Entity{{ID: "service:a", Name: "ServiceA"}}}
	if err := fbwriter.WriteAtomic(graphPath, doc); err != nil {
		t.Fatal(err)
	}

	group := "freshness-" + filepath.Base(root)
	cfgPath, err := registry.ConfigPathFor(group)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &registry.GroupConfig{Name: group, Repos: []registry.Repo{{Slug: "repo", Path: repoPath}}}
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddGroup(group, cfgPath); err != nil {
		t.Fatal(err)
	}
	return group, graphPath
}
