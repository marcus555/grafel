package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/graph/fbwriter"
	"github.com/cajasmota/grafel/internal/graph/groupalgo"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/testsupport"
)

func TestApplyGroupAlgorithmOverlay(t *testing.T) {
	grp := &DashGroup{Repos: map[string]*DashRepo{
		"api": {Doc: &graph.Document{Entities: []graph.Entity{
			{ID: "api:a", Name: "A"},
			{ID: "api:b", Name: "B"},
		}}},
		"web": {Doc: &graph.Document{Entities: []graph.Entity{
			{ID: "web:c", Name: "C"},
		}}},
	}}
	ov := &groupalgo.Overlay{
		Results: map[string]groupalgo.EntityOverlay{
			"api:a": {CommunityID: 7, PageRank: 0.8, Centrality: 0.4, IsGodNode: true},
			"api:b": {CommunityID: 7, PageRank: 0.2},
			"web:c": {CommunityID: 7, PageRank: 0.6, IsArticulationPoint: true},
		},
		Communities: []graph.CommunityResult{{ID: 7, Size: 3, AutoName: "shared-core"}},
		Stats:       graph.AlgorithmStats{NumCommunities: 1, NumGodNodes: 1},
	}

	applyGroupAlgorithmOverlay(grp, ov)

	a := grp.Repos["api"].Doc.Entities[0]
	if a.CommunityID == nil || *a.CommunityID != 7 || a.PageRank == nil || *a.PageRank != 0.8 || !a.IsGodNode {
		t.Fatalf("api:a overlay not restored: %+v", a)
	}
	c := grp.Repos["web"].Doc.Entities[0]
	if c.Centrality == nil || !c.IsArticulationPt {
		t.Fatalf("web:c overlay not restored: %+v", c)
	}
	apiCommunities := grp.Repos["api"].Doc.Communities
	webCommunities := grp.Repos["web"].Doc.Communities
	if len(apiCommunities) != 1 || apiCommunities[0].Size != 2 || apiCommunities[0].AutoName != "shared-core" {
		t.Fatalf("api community projection wrong: %+v", apiCommunities)
	}
	if len(webCommunities) != 1 || webCommunities[0].Size != 1 || webCommunities[0].TopEntities[0] != "web:c" {
		t.Fatalf("web community projection wrong: %+v", webCommunities)
	}
	if got := grp.Repos["api"].Doc.AlgorithmStats; got == nil || got.NumCommunities != 1 {
		t.Fatalf("algorithm stats not restored: %+v", got)
	}
}

func TestGraphCacheColdLoadRestoresPersistedOverlay(t *testing.T) {
	root := testsupport.IsolateHome(t)
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := daemon.StateDirForRepo(repoPath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := &graph.Document{Entities: []graph.Entity{{ID: "service:a", Name: "ServiceA"}}}
	if err := fbwriter.WriteAtomic(filepath.Join(stateDir, "graph.fb"), doc); err != nil {
		t.Fatalf("write graph.fb: %v", err)
	}

	const groupName = "cold-overlay"
	cfgPath, err := registry.ConfigPathFor(groupName)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &registry.GroupConfig{Name: groupName, Repos: []registry.Repo{{Slug: "repo", Path: repoPath}}}
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddGroup(groupName, cfgPath); err != nil {
		t.Fatal(err)
	}
	mtimes, err := groupalgo.CurrentSourceMtimes(groupName)
	if err != nil {
		t.Fatal(err)
	}
	ov := &groupalgo.Overlay{
		Group:        groupName,
		SourceMtimes: mtimes,
		Results: map[string]groupalgo.EntityOverlay{
			"service:a": {CommunityID: 4, PageRank: 0.91, Centrality: 0.72, IsGodNode: true},
		},
		Communities: []graph.CommunityResult{{ID: 4, Size: 1, AutoName: "service-core"}},
		Stats:       graph.AlgorithmStats{NumCommunities: 1},
	}
	if err := groupalgo.WriteOverlay(groupName, ov); err != nil {
		t.Fatal(err)
	}

	grp, err := NewGraphCache(0).loadGroup(groupName)
	if err != nil {
		t.Fatalf("cold load: %v", err)
	}
	defer closeDashGroupReaders(grp)
	entity := grp.Repos["repo"].Doc.Entities[0]
	if entity.PageRank == nil || *entity.PageRank != 0.91 || entity.CommunityID == nil || *entity.CommunityID != 4 {
		t.Fatalf("persisted overlay not restored: %+v", entity)
	}
	if got := grp.Repos["repo"].Doc.Communities; len(got) != 1 || got[0].AutoName != "service-core" {
		t.Fatalf("persisted communities not restored: %+v", got)
	}
}

func BenchmarkApplyGroupAlgorithmOverlay12K(b *testing.B) {
	const entityCount = 12500
	entities := make([]graph.Entity, entityCount)
	results := make(map[string]groupalgo.EntityOverlay, entityCount)
	communities := make([]graph.CommunityResult, 200)
	for i := range communities {
		communities[i] = graph.CommunityResult{ID: i, AutoName: fmt.Sprintf("community-%d", i)}
	}
	for i := range entities {
		id := fmt.Sprintf("entity-%05d", i)
		entities[i] = graph.Entity{ID: id, Name: id}
		results[id] = groupalgo.EntityOverlay{CommunityID: i % len(communities), PageRank: float64(entityCount - i)}
	}
	ov := &groupalgo.Overlay{Results: results, Communities: communities}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docEntities := append([]graph.Entity(nil), entities...)
		grp := &DashGroup{Repos: map[string]*DashRepo{"repo": {Doc: &graph.Document{Entities: docEntities}}}}
		applyGroupAlgorithmOverlay(grp, ov)
	}
}

// BenchmarkGraphCacheColdLoad measures a real registered group without making
// ordinary test runs depend on developer state. Set GRAFEL_LIVE_GROUP to opt in.
func BenchmarkGraphCacheColdLoad(b *testing.B) {
	groupName := os.Getenv("GRAFEL_LIVE_GROUP")
	if groupName == "" {
		b.Skip("set GRAFEL_LIVE_GROUP to benchmark a registered group")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		grp, err := NewGraphCache(0).loadGroup(groupName)
		if err != nil {
			b.Fatal(err)
		}
		closeDashGroupReaders(grp)
	}
}
