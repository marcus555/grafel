package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// buildBigOrientDoc synthesizes a repo Document with enough entities and
// cross-community relationships that graph.AnalyzeOrientation's default caps
// (top_entities=15, top_edges=15, max_questions=12) still produce a
// non-trivial block for that repo. Used to reproduce the #5783 overview
// overflow: many such repos in one group, each contributing its own
// per-repo-capped block, multiply into an unbounded total payload.
func buildBigOrientDoc(repo string, n int) *graph.Document {
	entities := make([]graph.Entity, 0, n)
	rels := make([]graph.Relationship, 0, n*2)
	for i := 0; i < n; i++ {
		community := i % 4
		entities = append(entities, graph.Entity{
			ID:          fmt.Sprintf("%s-e%d", repo, i),
			Name:        fmt.Sprintf("Entity%d", i),
			Kind:        "function",
			SourceFile:  fmt.Sprintf("src/pkg%d/file%d.go", community, i),
			CommunityID: ip(community),
			Centrality:  f64p(float64(i%100) / 100.0),
			PageRank:    f64p(float64(i%50) / 100.0),
		})
		if i > 0 {
			// Wire every entity to its predecessor, and every 3rd entity
			// across to a different community so cross_cutting_edges has
			// plenty of bridge candidates to rank and cap.
			rels = append(rels, graph.Relationship{
				FromID: fmt.Sprintf("%s-e%d", repo, i),
				ToID:   fmt.Sprintf("%s-e%d", repo, i-1),
				Kind:   "CALLS",
			})
			if i%3 == 0 {
				rels = append(rels, graph.Relationship{
					FromID: fmt.Sprintf("%s-e%d", repo, i),
					ToID:   fmt.Sprintf("%s-e%d", repo, (i+n/2)%n),
					Kind:   "CALLS",
				})
			}
		}
	}
	return &graph.Document{Repo: repo, Entities: entities, Relationships: rels}
}

// Test_handleOrient_Overview_Bounded_5783 reproduces the live overflow: a
// group with many repos, each producing its own default-capped orientation
// block (15 key entities + 15 cross-cutting edges + 12 questions), still
// multiplies unboundedly across repos because nothing bounds the OVERALL
// overview payload. On unfixed code this assembles well past any reasonable
// single-line MCP result size. The fix must keep the result under a bounded
// byte ceiling regardless of repo count, while still honoring top_entities/
// top_edges on every block that IS included.
func Test_handleOrient_Overview_Bounded_5783(t *testing.T) {
	const numRepos = 40
	docs := make([]*graph.Document, 0, numRepos)
	for i := 0; i < numRepos; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 60))
	}
	srv := newTestServer(t, docs...)

	text := callEndpointToolText(t, srv.handleOrient, map[string]any{
		"group":        "test",
		"top_entities": 5,
		"top_edges":    5,
	})
	if text == "" {
		t.Fatal("empty result")
	}

	const boundedCeilingBytes = 32 * 1024 // well under any MCP tool-result token limit
	if len(text) > boundedCeilingBytes {
		t.Fatalf("overview result is %d bytes, want <= %d bytes (unbounded overview overflow, #5783)",
			len(text), boundedCeilingBytes)
	}

	var got []map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v\nbody (first 500 bytes)=%s", err, firstN(text, 500))
	}

	// top_entities/top_edges must be honored on every real repo block
	// (a synthetic truncation-marker block, if present, has no key_entities
	// field and is skipped).
	for _, block := range got {
		ke, ok := block["key_entities"].([]any)
		if !ok {
			continue // truncation marker block
		}
		if len(ke) > 5 {
			t.Errorf("repo %v: top_entities=5 not honored, got %d key_entities", block["repo"], len(ke))
		}
		cc, _ := block["cross_cutting_edges"].([]any)
		if len(cc) > 5 {
			t.Errorf("repo %v: top_edges=5 not honored, got %d cross_cutting_edges", block["repo"], len(cc))
		}
	}

	// With 40 repos and a default token_budget, some repos must have been
	// omitted — the result must carry an explicit, non-silent truncation
	// indicator rather than quietly dropping data.
	foundMarker := false
	for _, block := range got {
		if t, _ := block["truncated"].(bool); t {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Errorf("expected a truncated:true marker block signalling omitted repos, got none (result had %d blocks)", len(got))
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
