package mcp

import (
	"context"
	"fmt"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"

	"github.com/cajasmota/grafel/internal/graph"
)

// buildBigOrientDoc synthesizes a repo Document with enough entities and
// cross-community relationships that graph.AnalyzeOrientation's default caps
// (top_entities=15, top_edges=15, max_questions=12) produce a full,
// event-driven-ai-shaped block for that repo (~15 key_entities + ~15
// cross_cutting_edges + ~12 questions). Used to reproduce the #5783 overview
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

// orientSafeCeilingBytes is the safe ceiling for the FINAL wire body. The
// client rejected ~74k chars; we assert the delivered body is comfortably
// under this. #5783 v0.1.8.1.
const orientSafeCeilingBytes = 55 * 1000

// callOrientWire invokes handleOrient and returns the REAL final wire body the
// client receives — the marshaled {count,elapsed_ms,items} envelope with the
// TOON-encoded `items` schema-string — produced through the SAME
// finalizeDeferred path wrap() uses. This is the crux of #5783: the prior test
// measured res.Content[0].Text (the eager json.Marshal from jsonResult), which
// is ~15% SMALLER than the delivered body because it skips the TOON→JSON
// double-escape. Measuring the eager layer passed while reality overflowed.
//
// It also returns the repo blocks (pre-serialization) so tests can count repos
// and inspect per-repo detail without re-parsing the TOON string.
func callOrientWire(t *testing.T, srv *Server, args map[string]any) (wireBody string, repos []map[string]any, marker map[string]any) {
	t.Helper()
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = args
	res, err := srv.handleOrient(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("handler returned error result: %+v", res)
	}
	dv, ok := takeDeferred(res)
	if !ok {
		t.Fatal("handler did not stash a deferred JSON payload")
	}
	blocks, ok := dv.([]map[string]any)
	if !ok {
		t.Fatalf("deferred payload is %T, want []map[string]any", dv)
	}
	body, ferr := finalizeDeferred(blocks, 0, nil)
	if ferr != nil {
		t.Fatalf("finalizeDeferred: %v", ferr)
	}
	for _, b := range blocks {
		if tr, _ := b["truncated"].(bool); tr {
			marker = b
			continue
		}
		repos = append(repos, b)
	}
	return body, repos, marker
}

// keCount returns the key_entities length of a repo block, tolerating both the
// typed slice (pre-serialization) and a decoded []any.
func keCount(block map[string]any) int {
	switch ke := block["key_entities"].(type) {
	case []graph.KeyEntity:
		return len(ke)
	case []any:
		return len(ke)
	default:
		return 0
	}
}

// Test_handleOrient_Overview_WireBounded_5783 is the crux test: at REAL default
// settings a 9-repo event-driven-ai-shaped group must produce a FINAL WIRE body
// (measured through finalizeDeferred, the delivered path) under the safe
// ceiling AND still include all 9 repos. The pre-fix code measured only the
// eager-marshal layer (~64k) and passed while the delivered body was ~74k and
// overflowed the client — so this test measures the real body.
func Test_handleOrient_Overview_WireBounded_5783(t *testing.T) {
	const numRepos = 9
	docs := make([]*graph.Document, 0, numRepos)
	for i := 0; i < numRepos; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 200))
	}
	srv := newTestServer(t, docs...)

	// REAL production defaults — no cap overrides at all.
	body, repos, marker := callOrientWire(t, srv, map[string]any{"group": "test"})

	if len(body) > orientSafeCeilingBytes {
		t.Fatalf("FINAL wire body is %d bytes, want <= %d (delivered response must stay under the client limit, #5783)",
			len(body), orientSafeCeilingBytes)
	}
	if len(repos) != numRepos {
		t.Fatalf("expected all %d repos in overview, got %d (marker=%v) — repos must scale detail, not be dropped",
			numRepos, len(repos), marker)
	}
	nonEmpty := 0
	for _, r := range repos {
		if keCount(r) > 0 {
			nonEmpty++
		}
	}
	if nonEmpty < numRepos {
		t.Errorf("expected every repo to carry key_entities detail at default budget, only %d/%d did", nonEmpty, numRepos)
	}
}

// Test_handleOrient_Overview_TokenBudgetHardCap_5783 asserts token_budget is a
// genuine hard cap on the FINAL wire body: token_budget=N ⇒ delivered body
// ≲ N*4 bytes.
func Test_handleOrient_Overview_TokenBudgetHardCap_5783(t *testing.T) {
	docs := make([]*graph.Document, 0, 9)
	for i := 0; i < 9; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 200))
	}
	srv := newTestServer(t, docs...)

	for _, tb := range []int{3000, 6000, 12000} {
		body, repos, _ := callOrientWire(t, srv, map[string]any{
			"group":        "test",
			"token_budget": tb,
		})
		if len(body) > tb*4 {
			t.Errorf("token_budget=%d: FINAL wire body %d bytes exceeds hard cap %d", tb, len(body), tb*4)
		}
		if len(repos) != 9 {
			t.Errorf("token_budget=%d: expected all 9 repos, got %d", tb, len(repos))
		}
	}
}

// Test_handleOrient_Overview_DetailShrinks_5783 asserts per-repo detail shrinks
// as the group grows: the same repo shows fewer key entities in a large group
// than in a small one, because the shared budget spreads across more repos. All
// repos remain present in both cases, measured against the real wire body.
func Test_handleOrient_Overview_DetailShrinks_5783(t *testing.T) {
	makeGroup := func(numRepos int) *Server {
		docs := make([]*graph.Document, 0, numRepos)
		for i := 0; i < numRepos; i++ {
			docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-%02d", i), 200))
		}
		return newTestServer(t, docs...)
	}

	smallBody, smallRepos, _ := callOrientWire(t, makeGroup(3), map[string]any{"group": "test"})
	largeBody, largeRepos, largeMarker := callOrientWire(t, makeGroup(30), map[string]any{"group": "test"})

	if len(smallRepos) != 3 {
		t.Fatalf("small group: want 3 repos, got %d", len(smallRepos))
	}
	if len(largeRepos) != 30 {
		t.Fatalf("large group: want all 30 repos, got %d (marker=%v)", len(largeRepos), largeMarker)
	}
	if len(smallBody) > orientSafeCeilingBytes || len(largeBody) > orientSafeCeilingBytes {
		t.Fatalf("wire bodies must stay under ceiling: small=%d large=%d ceiling=%d", len(smallBody), len(largeBody), orientSafeCeilingBytes)
	}

	if largeDetail, smallDetail := keCount(largeRepos[0]), keCount(smallRepos[0]); largeDetail >= smallDetail {
		t.Errorf("per-repo detail did not shrink as group grew: 3-repo group=%d key_entities/repo, 30-repo group=%d (want fewer in the larger group)",
			smallDetail, largeDetail)
	}
}

// Test_handleOrient_Overview_MarkerBounded_5783 forces the last-resort drop
// path (a pathological group at a tiny budget) and asserts the delivered wire
// body — marker included — stays under the budget and the marker's
// omitted_repos list is itself bounded, so a 200-repo probe cannot blow the
// budget via an unbounded name list.
func Test_handleOrient_Overview_MarkerBounded_5783(t *testing.T) {
	const numRepos = 200
	docs := make([]*graph.Document, 0, numRepos)
	for i := 0; i < numRepos; i++ {
		docs = append(docs, buildBigOrientDoc(fmt.Sprintf("repo-with-a-fairly-long-name-%03d", i), 40))
	}
	srv := newTestServer(t, docs...)

	// Tiny token_budget forces the drop path even for name-only blocks.
	tb := 1000
	body, _, marker := callOrientWire(t, srv, map[string]any{
		"group":        "test",
		"token_budget": tb,
	})
	if len(body) > tb*4 {
		t.Fatalf("FINAL wire body is %d bytes, want <= %d even in the truncation path (#5783)", len(body), tb*4)
	}
	if marker == nil {
		t.Fatal("expected a truncated:true marker when the group cannot fully fit")
	}
	omitted, _ := marker["omitted_repos"].([]string)
	if len(omitted) > 20 {
		t.Errorf("marker omitted_repos must be capped (<=20 names), got %d", len(omitted))
	}
	if oc, ok := marker["omitted_count"].(int); !ok || oc < len(omitted) {
		t.Errorf("marker must carry an omitted_count >= shown names, got %v", marker["omitted_count"])
	}
}
