package mcp

// messaging_5782_test.go — grafel #5782 Phase 1 (query-layer, framework-agnostic).
//
// Three cross-repo message-topic gaps, all in the MCP query layer over graph
// data that ALREADY exists (per-repo SCOPE.MessageTopic entities keyed by a
// broker-prefixed Name, intra-repo PUBLISHES_TO / SUBSCRIBES_TO / DELIVERS_TO
// edges, and cross-repo topic joins in lg.Links):
//
//   #3  grafel_related direction=messaging returns a topic's producers +
//       consumers ACROSS repos (was: empty, because the neighbor handlers
//       return on the first repo containing the resolved entity).
//   #2  grafel_impact_radius on a topic includes cross-repo consumers in other
//       repos (was: intra-repo only — walked one resolved repo's adjacency).
//   bug grafel_debt kind=stubs without group_v3+group_oracle returns a clear
//       "two-group comparison mode" gated error, not the bare
//       "group_v3 and group_oracle are both required".

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// hasFold reports whether s contains sub, case-insensitively.
func hasFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// newMessagingTestServer builds a two-repo group with the SAME broker-prefixed
// topic Name appearing as a per-repo SCOPE.MessageTopic entity in each repo:
//
//	repo "producer": OrderService.placeOrder --PUBLISHES_TO--> kafka:orders.placed
//	repo "consumer": InventoryConsumer.onOrder --SUBSCRIBES_TO--> kafka:orders.placed
//	                 kafka:orders.placed --DELIVERS_TO--> InventoryConsumer.onOrder
//
// plus the cross-repo topic join in lg.Links (Method="topic") the topic pass
// (internal/links/topic_pass.go P7) writes: publisher-entity → subscriber-entity.
func newMessagingTestServer(t *testing.T) *Server {
	t.Helper()

	const topicName = "kafka:orders.placed"

	producerDoc := minDoc(
		[]graph.Entity{
			{ID: "op:placeOrder", Name: "OrderService.placeOrder", Kind: "SCOPE.Operation", SourceFile: "order_service.go", StartLine: 12, QualifiedName: "OrderService.placeOrder"},
			{ID: "topic:orders", Name: topicName, Kind: "SCOPE.MessageTopic", SourceFile: ""},
		},
		[]graph.Relationship{
			{ID: "pub:1", FromID: "op:placeOrder", ToID: "topic:orders", Kind: "PUBLISHES_TO"},
		},
	)
	producerDoc.Repo = "producer"

	consumerDoc := minDoc(
		[]graph.Entity{
			{ID: "op:onOrder", Name: "InventoryConsumer.onOrder", Kind: "SCOPE.Operation", SourceFile: "inventory_consumer.go", StartLine: 30, QualifiedName: "InventoryConsumer.onOrder"},
			{ID: "topic:orders", Name: topicName, Kind: "SCOPE.MessageTopic", SourceFile: ""},
		},
		[]graph.Relationship{
			{ID: "sub:1", FromID: "op:onOrder", ToID: "topic:orders", Kind: "SUBSCRIBES_TO"},
			{ID: "del:1", FromID: "topic:orders", ToID: "op:onOrder", Kind: "DELIVERS_TO", Properties: map[string]string{"trigger": "async", "broker": "kafka"}},
		},
	)
	consumerDoc.Repo = "consumer"

	reg := &Registry{Groups: map[string]RegistryGroup{
		"test": {Repos: map[string]RegistryRepo{
			"producer": {Path: t.TempDir()},
			"consumer": {Path: t.TempDir()},
		}},
	}}
	st := NewState(reg)
	st.mu.Lock()
	lg := &LoadedGroup{
		Name: "test",
		Repos: map[string]*LoadedRepo{
			"producer": {Repo: "producer", Doc: producerDoc, LabelIndex: BuildLabelIndex(producerDoc)},
			"consumer": {Repo: "consumer", Doc: consumerDoc, LabelIndex: BuildLabelIndex(consumerDoc)},
		},
		Links: []CrossRepoLink{
			{
				Source:     "producer::op:placeOrder",
				Target:     "consumer::op:onOrder",
				Relation:   "publishes_to",
				Method:     "topic",
				Channel:    "kafka",
				Identifier: topicName,
			},
		},
	}
	st.groups["test"] = lg
	st.mu.Unlock()

	return &Server{State: st, Tel: NewTelemetry(0)}
}

// collectNames flattens a []any of neighbor maps into a repo-keyed name set.
func collectNames(rows any) map[string]string {
	out := map[string]string{}
	rr, _ := rows.([]any)
	for _, r := range rr {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		repo, _ := m["repo"].(string)
		out[name] = repo
	}
	return out
}

// ---------------------------------------------------------------------------
// ITEM #3 — grafel_related direction=messaging
// ---------------------------------------------------------------------------

// TestMessagingRelated_CrossRepoProducersConsumers is the #3 assertion:
// direction=messaging on a topic surfaces its producers AND consumers even when
// they live in sibling repos. Resolved by topic Name.
func TestMessagingRelated_CrossRepoProducersConsumers(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "messaging",
		"entity_id": "kafka:orders.placed",
	})
	if out == nil {
		t.Fatal("nil/non-JSON result")
	}

	producers := collectNames(out["producers"])
	consumers := collectNames(out["consumers"])

	if repo := producers["OrderService.placeOrder"]; repo != "producer" {
		t.Errorf("expected producer OrderService.placeOrder in repo producer, got producers=%+v", producers)
	}
	if repo := consumers["InventoryConsumer.onOrder"]; repo != "consumer" {
		t.Errorf("expected consumer InventoryConsumer.onOrder in repo consumer, got consumers=%+v", consumers)
	}
}

// TestMessagingRelated_ResolveByPrefixedID resolves the topic by its prefixed
// entity_id (repo::localID) rather than by Name, and still folds the sibling
// repo's subscriber.
func TestMessagingRelated_ResolveByPrefixedID(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "messaging",
		"entity_id": "producer::topic:orders",
	})
	if out == nil {
		t.Fatal("nil/non-JSON result")
	}
	consumers := collectNames(out["consumers"])
	if _, ok := consumers["InventoryConsumer.onOrder"]; !ok {
		t.Errorf("expected cross-repo consumer InventoryConsumer.onOrder, got consumers=%+v", consumers)
	}
}

// TestCoreRelated_ExistingDirectionsUnchanged is the guard: adding the
// messaging direction must not break the existing discriminator values.
func TestCoreRelated_ExistingDirectionsUnchanged(t *testing.T) {
	srv := newTestServer(t, buildChainDoc())
	// direction=callers on FuncB should still return FuncA.
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "callers",
		"entity_id": "ent-b",
	})
	callers, _ := out["callers"].([]any)
	if len(callers) == 0 {
		t.Fatalf("callers direction regressed: expected FuncA, got %+v", out)
	}
}

// ---------------------------------------------------------------------------
// ITEM #2 — grafel_impact_radius on a topic
// ---------------------------------------------------------------------------

// TestImpactRadius_TopicCrossRepoConsumer is the #2 assertion: the blast radius
// of a topic must include a cross-repo consumer that subscribes in ANOTHER repo.
func TestImpactRadius_TopicCrossRepoConsumer(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleImpactRadius, map[string]any{
		"entity_id": "producer::topic:orders",
		"hops":      float64(2),
	})
	if out == nil {
		t.Fatal("nil result")
	}
	names := collectNames(out["affected"])
	if repo := names["InventoryConsumer.onOrder"]; repo != "consumer" {
		t.Errorf("expected cross-repo consumer InventoryConsumer.onOrder (repo consumer) in blast radius, got affected=%+v", names)
	}
	// The local publisher must remain in the radius too.
	if _, ok := names["OrderService.placeOrder"]; !ok {
		t.Errorf("expected local publisher OrderService.placeOrder in blast radius, got affected=%+v", names)
	}
}

// TestImpactRadius_NonTopicUnchanged guards the code-entity path: a plain CALLS
// chain still resolves intra-repo without regression.
func TestImpactRadius_NonTopicUnchanged(t *testing.T) {
	srv := newTestServer(t, buildChainDoc())
	// FuncC is called by FuncB, which is called by FuncA — both are affected.
	out := callFlowTool(t, srv.handleImpactRadius, map[string]any{
		"entity_id": "ent-c",
		"hops":      float64(2),
	})
	names := collectNames(out["affected"])
	if _, ok := names["FuncB"]; !ok {
		t.Errorf("non-topic impact regressed: expected FuncB affected, got %+v", names)
	}
	if _, ok := names["FuncA"]; !ok {
		t.Errorf("non-topic impact regressed: expected FuncA affected, got %+v", names)
	}
}

// ---------------------------------------------------------------------------
// bug — grafel_debt kind=stubs gating
// ---------------------------------------------------------------------------

// TestDebtStubs_GatedErrorWithoutGroups asserts the clear two-group gate error.
func TestDebtStubs_GatedErrorWithoutGroups(t *testing.T) {
	srv := newTestServer(t, buildChainDoc())
	msg := callFlowToolError(t, srv.handleAnalysisDebt, map[string]any{
		"kind": "stubs",
	})
	for _, want := range []string{"group_v3", "group_oracle", "two-group"} {
		if !hasFold(msg, want) {
			t.Errorf("stub gate error must mention %q; got: %s", want, msg)
		}
	}
}

// TestDebtStubs_BothGroupsPassesGate asserts that supplying both group args
// clears the gate (the two-group path still runs; here it fails later with a
// distinct "not loaded" error, proving the gate no longer fires).
func TestDebtStubs_BothGroupsPassesGate(t *testing.T) {
	srv := newTestServer(t, buildChainDoc())
	msg := callFlowToolError(t, srv.handleAnalysisDebt, map[string]any{
		"kind":         "stubs",
		"group_v3":     "nope_v3",
		"group_oracle": "nope_oracle",
	})
	if hasFold(msg, "two-group") {
		t.Errorf("gate should not fire when both groups are provided; got: %s", msg)
	}
	if !hasFold(msg, "not loaded") {
		t.Errorf("expected the two-group path to proceed to a load error, got: %s", msg)
	}
}
