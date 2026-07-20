package mcp

// impact_budget_5793_test.go — #5793: grafel_impact_radius must be compact by
// default and honour a token_budget as a HARD cap on the REAL wire body.
//
// Same bug class as #5783 (orient overview overflow). The crux, learned there
// and re-applied here: the cap MUST be measured against the FINAL serialized
// response produced by finalizeDeferred (the delivered path), not against an
// intermediate json.Marshal of the affected array. callImpactWire below
// reproduces the delivered body exactly.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"

	"github.com/cajasmota/grafel/internal/graph"
)

// buildHighDegreeImpactDoc builds a single hub entity with n inbound callers so
// impact_radius(hub) yields n affected entities — a pathological high-degree
// node whose verbose per-entity dump previously overflowed the MCP token cap.
func buildHighDegreeImpactDoc(n int) *graph.Document {
	entities := []graph.Entity{
		{ID: "ent-hub", Name: "hub", Kind: "Function", SourceFile: "hub.go"},
	}
	var rels []graph.Relationship
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("caller-%d", i)
		entities = append(entities, graph.Entity{
			ID: id, Name: fmt.Sprintf("caller%d", i), Kind: "Function",
			SourceFile: fmt.Sprintf("pkg/callers/file%d.go", i),
		})
		rels = append(rels, graph.Relationship{FromID: id, ToID: "ent-hub", Kind: "CALLS"})
	}
	return minDoc(entities, rels)
}

// callImpactWire invokes handleImpactRadius and returns the REAL final wire body
// the client receives — produced through the SAME finalizeDeferred path wrap()
// uses — plus the parsed body. This is the crux of #5793 (and #5783 before it):
// measuring the delivered body, not the eager json.Marshal of the affected
// slice.
func callImpactWire(t *testing.T, srv *Server, args map[string]any) (string, map[string]any) {
	t.Helper()
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = args
	res, err := srv.handleImpactRadius(context.Background(), req)
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
	out, ok := dv.(map[string]any)
	if !ok {
		t.Fatalf("deferred payload is %T, want map[string]any", dv)
	}
	body, ferr := finalizeDeferred(out, 0, nil)
	if ferr != nil {
		t.Fatalf("finalizeDeferred: %v", ferr)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("wire body is not JSON: %v\n%s", err, body)
	}
	return body, parsed
}

// Test_ImpactRadius_CompactDefault_Inline_5793: a high-degree node's DEFAULT
// response FINAL wire body stays comfortably inline, and carries the compact
// aggregate fields (total_affected + per-kind breakdown + hop distribution)
// rather than a full per-entity dump.
func Test_ImpactRadius_CompactDefault_Inline_5793(t *testing.T) {
	const n = 800
	srv := newTestServer(t, buildHighDegreeImpactDoc(n))

	body, out := callImpactWire(t, srv, map[string]any{
		"entity_id": "ent-hub",
		"hops":      float64(1),
	})

	// Default budget is conservative (=> a small byte ceiling). The delivered
	// body must be a genuine hard cap on that.
	if len(body) > impactDefaultBudget(false)*4 {
		t.Fatalf("default FINAL wire body is %d bytes, want <= %d (must stay inline for high-degree nodes, #5793)",
			len(body), impactDefaultBudget(false)*4)
	}

	// total_affected must reflect the FULL blast radius, not the shown slice.
	total, _ := out["total_affected"].(float64)
	if int(total) != n {
		t.Errorf("total_affected = %v, want %d (aggregate must count the whole radius)", out["total_affected"], n)
	}

	// per-kind breakdown must be present and sum to total_affected.
	breakdown, ok := out["breakdown"].(map[string]any)
	if !ok || len(breakdown) == 0 {
		t.Fatalf("expected non-empty breakdown map, got %T %v", out["breakdown"], out["breakdown"])
	}
	sum := 0
	for _, v := range breakdown {
		c, _ := v.(float64)
		sum += int(c)
	}
	if sum != n {
		t.Errorf("breakdown sums to %d, want %d (breakdown must cover all affected)", sum, n)
	}

	// hop distribution present.
	if hd, ok := out["hop_distribution"].(map[string]any); !ok || len(hd) == 0 {
		t.Errorf("expected non-empty hop_distribution, got %T %v", out["hop_distribution"], out["hop_distribution"])
	}

	// The default shows only a SMALL top-N of entities, not the full dump.
	affected, _ := out["affected"].([]any)
	if len(affected) > impactCompactTopN {
		t.Errorf("default affected list has %d entries, want <= %d (compact top-N)", len(affected), impactCompactTopN)
	}
	if len(affected) == 0 {
		t.Error("default should still surface a small top-N of affected entities")
	}
}

// Test_ImpactRadius_TokenBudget_HardCap_5793: token_budget is a genuine hard cap
// on the delivered body; a smaller budget yields a no-larger real body, and the
// aggregate (total_affected + breakdown) always survives the shrink.
func Test_ImpactRadius_TokenBudget_HardCap_5793(t *testing.T) {
	const n = 800
	srv := newTestServer(t, buildHighDegreeImpactDoc(n))

	var lastLen int
	first := true
	for _, tb := range []int{500, 1000, 3000} {
		body, out := callImpactWire(t, srv, map[string]any{
			"entity_id":    "ent-hub",
			"hops":         float64(1),
			"detail":       "full", // exercise the cap on the verbose path too
			"token_budget": tb,
		})
		if len(body) > tb*4 {
			t.Errorf("token_budget=%d: FINAL wire body %d bytes exceeds hard cap %d", tb, len(body), tb*4)
		}
		// Aggregate must survive even at the smallest budget.
		if total, _ := out["total_affected"].(float64); int(total) != n {
			t.Errorf("token_budget=%d: total_affected=%v, want %d (aggregate must not shrink away)", tb, out["total_affected"], n)
		}
		if b, ok := out["breakdown"].(map[string]any); !ok || len(b) == 0 {
			t.Errorf("token_budget=%d: breakdown must survive the shrink", tb)
		}
		if !first && len(body) < lastLen {
			t.Errorf("token_budget ordering: budget=%d body=%d unexpectedly smaller than previous %d", tb, len(body), lastLen)
		}
		first = false
		lastLen = len(body)
	}
}

// Test_ImpactRadius_FullDetail_TruncationNote_5793: the verbose per-entity list
// is reachable via detail=full, and the historical 500-cap truncation_note
// still fires when the full list is capped (given a budget large enough that the
// 500-cap — not the byte budget — is the binding constraint).
func Test_ImpactRadius_FullDetail_TruncationNote_5793(t *testing.T) {
	n := impactRadiusMaxResults + 120
	srv := newTestServer(t, buildHighDegreeImpactDoc(n))

	_, out := callImpactWire(t, srv, map[string]any{
		"entity_id":    "ent-hub",
		"hops":         float64(1),
		"detail":       "full",
		"token_budget": 400000, // large enough that the 500-cap binds, not bytes
	})

	affected, ok := out["affected"].([]any)
	if !ok {
		t.Fatalf("expected affected array, got %T", out["affected"])
	}
	if len(affected) != impactRadiusMaxResults {
		t.Errorf("full detail affected len = %d, want %d (500-cap)", len(affected), impactRadiusMaxResults)
	}
	if out["truncated"] != true {
		t.Errorf("expected truncated=true when full list is capped, got %v", out["truncated"])
	}
	if note, _ := out["truncation_note"].(string); note == "" {
		t.Error("expected a non-empty truncation_note")
	}
	if total, _ := out["total_affected"].(float64); int(total) != n {
		t.Errorf("total_affected=%v, want %d", out["total_affected"], n)
	}
}
