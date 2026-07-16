package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// buildBigOrientDoc synthesizes a repo Document with enough entities and
// cross-community relationships that graph.AnalyzeOrientation's default caps
// (top_entities=15, top_edges=15, max_questions=12) produce a full,
// non-trivial block for that repo. Used to reproduce the #5783 overview
// overflow and to verify the adaptive-detail redesign.
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

const orientCeilingBytes = 64 * 1024 // grafel_orient hard byte ceiling (#5783)

// orientRepoBlocks unmarshals an overview result and splits it into real repo
// blocks (those carrying key_entities) and any truncation-marker block.
func orientRepoBlocks(t *testing.T, text string) (repos []map[string]any, marker map[string]any) {
	t.Helper()
	var got []map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v\nbody (first 500 bytes)=%s", err, firstN(text, 500))
	}
	for _, block := range got {
		if tr, _ := block["truncated"].(bool); tr {
			marker = block
			continue
		}
		repos = append(repos, block)
	}
	return repos, marker
}

// Test_handleOrient_Overview_AllRepos_5783 asserts that at REAL default
// settings (top_entities=15/top_edges=15/max_questions=12, no token_budget
// override) a 9-repo group — exactly event-driven-ai, the group that overflowed
// — surfaces ALL 9 repos with reduced per-repo detail, staying under the 64KB
// ceiling. The pre-redesign behavior dropped 7 of 9 repos behind a marker and
// would fail the all-9 assertion.
func Test_handleOrient_Overview_AllRepos_5783(t *testing.T) {
	const numRepos = 9
	docs := make([]*graph.Document, 0, numRepos)
	for i := 0; i < numRepos; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 120))
	}
	srv := newTestServer(t, docs...)

	// REAL production defaults — no cap overrides at all.
	text := callEndpointToolText(t, srv.handleOrient, map[string]any{"group": "test"})
	if text == "" {
		t.Fatal("empty result")
	}
	if len(text) > orientCeilingBytes {
		t.Fatalf("overview result is %d bytes, want <= %d (must stay under ceiling, #5783)", len(text), orientCeilingBytes)
	}

	repos, marker := orientRepoBlocks(t, text)
	if len(repos) != numRepos {
		t.Fatalf("expected all %d repos in overview, got %d (marker=%v) — repos must scale detail, not be dropped",
			numRepos, len(repos), marker)
	}
	// Every repo should still carry SOME orientation content at default budget.
	nonEmpty := 0
	for _, r := range repos {
		ke, _ := r["key_entities"].([]any)
		if len(ke) > 0 {
			nonEmpty++
		}
	}
	if nonEmpty < numRepos {
		t.Errorf("expected every repo to carry key_entities detail at default budget, only %d/%d did", nonEmpty, numRepos)
	}
}

// Test_handleOrient_Overview_DetailShrinks_5783 asserts the per-repo detail
// visibly shrinks as the group grows: the same repo shows fewer key entities
// in a large group than in a small one, because the shared byte budget is
// divided across more repos. All repos remain present in both cases.
func Test_handleOrient_Overview_DetailShrinks_5783(t *testing.T) {
	makeGroup := func(numRepos int) string {
		docs := make([]*graph.Document, 0, numRepos)
		for i := 0; i < numRepos; i++ {
			docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 120))
		}
		srv := newTestServer(t, docs...)
		return callEndpointToolText(t, srv.handleOrient, map[string]any{"group": "test"})
	}

	smallRepos, _ := orientRepoBlocks(t, makeGroup(3))
	largeRepos, largeMarker := orientRepoBlocks(t, makeGroup(30))

	if len(smallRepos) != 3 {
		t.Fatalf("small group: want 3 repos, got %d", len(smallRepos))
	}
	// A 30-repo group must still list all 30 repos at minimal detail.
	if len(largeRepos) != 30 {
		t.Fatalf("large group: want all 30 repos, got %d (marker=%v)", len(largeRepos), largeMarker)
	}

	keCount := func(blocks []map[string]any) int {
		ke, _ := blocks[0]["key_entities"].([]any)
		return len(ke)
	}
	smallDetail := keCount(smallRepos)
	largeDetail := keCount(largeRepos)
	if largeDetail >= smallDetail {
		t.Errorf("per-repo detail did not shrink as group grew: 3-repo group=%d key_entities/repo, 30-repo group=%d (want fewer in the larger group)",
			smallDetail, largeDetail)
	}
}

// Test_handleOrient_Overview_MarkerBounded_5783 drives a pathological group
// large enough that even minimal per-repo blocks cannot all fit, forcing the
// last-resort truncation path — and asserts the result (marker included) stays
// under the ceiling and the marker's omitted_repos list is itself bounded (so a
// 200-repo probe cannot blow the budget via an unbounded name list).
func Test_handleOrient_Overview_MarkerBounded_5783(t *testing.T) {
	const numRepos = 200
	docs := make([]*graph.Document, 0, numRepos)
	for i := 0; i < numRepos; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-with-a-fairly-long-name-%03d", i), 40))
	}
	srv := newTestServer(t, docs...)

	// Tiny token_budget forces the last-resort drop path even for modest repos.
	text := callEndpointToolText(t, srv.handleOrient, map[string]any{
		"group":        "test",
		"token_budget": 2000,
	})
	if text == "" {
		t.Fatal("empty result")
	}
	if len(text) > orientCeilingBytes {
		t.Fatalf("result is %d bytes, want <= %d even in the truncation path (#5783)", len(text), orientCeilingBytes)
	}

	_, marker := orientRepoBlocks(t, text)
	if marker == nil {
		t.Fatal("expected a truncated:true marker when the group cannot fully fit")
	}
	omitted, _ := marker["omitted_repos"].([]any)
	if len(omitted) > 20 {
		t.Errorf("marker omitted_repos must be capped (<=20 names), got %d", len(omitted))
	}
	// omitted_count must still report the true total omitted, even though the
	// name list is truncated.
	if oc, ok := marker["omitted_count"].(float64); !ok || int(oc) < len(omitted) {
		t.Errorf("marker must carry an omitted_count >= shown names, got %v", marker["omitted_count"])
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
