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
			{
				ID: "cb:orders-out", Name: "orders-out", Kind: "SCOPE.ChannelBinding", SourceFile: "application.properties", StartLine: 1,
				Properties: map[string]string{"channel": "orders-out", "direction": "outgoing", "connector": "smallrye-kafka", "topic": topicName},
			},
		},
		[]graph.Relationship{
			{ID: "pub:1", FromID: "op:placeOrder", ToID: "topic:orders", Kind: "PUBLISHES_TO"},
			{ID: "bindch:1", FromID: "cb:orders-out", ToID: "op:placeOrder", Kind: "BINDS_CHANNEL"},
			{ID: "bindtop:1", FromID: "cb:orders-out", ToID: "topic:orders", Kind: "BINDS_TOPIC"},
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
// #5782 (issue #5782 asks #3 / #5) — agent-facing query-surface gaps.
// ---------------------------------------------------------------------------

// TestRelated_NeighborsDefaultResolvesMessageTopic is fix A: an agent that
// calls grafel_related with NO direction (or direction=neighbors) on a
// SCOPE.MessageTopic must not see the empty {callers:[],callees:[]} the
// generic CALLS-only neighbors handler returns for a topic — it should get
// the same cross-repo producers/consumers/handlers view as direction=messaging.
func TestRelated_NeighborsDefaultResolvesMessageTopic(t *testing.T) {
	srv := newMessagingTestServer(t)

	// direction=neighbors — an agent's natural "what's connected to this?"
	// call, previously an empty {callers:[],callees:[]} for a topic.
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "neighbors",
		"entity_id": "kafka:orders.placed",
	})
	if got, _ := out["direction"].(string); got != "neighbors" {
		t.Errorf("expected direction=neighbors echoed back (not messaging), got %q: %+v", got, out)
	}
	producers := collectNames(out["producers"])
	consumers := collectNames(out["consumers"])
	if repo := producers["OrderService.placeOrder"]; repo != "producer" {
		t.Errorf("direction=neighbors on a MessageTopic must surface producers, got %+v", out)
	}
	if repo := consumers["InventoryConsumer.onOrder"]; repo != "consumer" {
		t.Errorf("direction=neighbors on a MessageTopic must surface cross-repo consumers, got %+v", out)
	}
	if n, _ := out["count"].(float64); n <= 0 {
		t.Errorf("direction=neighbors on a MessageTopic must not return an empty result, got %+v", out)
	}

	// direction=both takes the same path (the two discriminator values are
	// synonyms upstream of the messaging-aware resolution).
	out2 := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "both",
		"entity_id": "kafka:orders.placed",
	})
	if n, _ := out2["count"].(float64); n <= 0 {
		t.Errorf("direction=both on a MessageTopic must not return an empty result, got %+v", out2)
	}
}

// TestRelated_NeighborsDefaultResolvesChannelBinding is fix A's ChannelBinding
// case: direction=neighbors on a SCOPE.ChannelBinding surfaces its bound
// channel (BINDS_CHANNEL) and topic (BINDS_TOPIC).
func TestRelated_NeighborsDefaultResolvesChannelBinding(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "neighbors",
		"entity_id": "producer::cb:orders-out",
	})
	if got, _ := out["kind"].(string); got != "channel_binding" {
		t.Fatalf("expected kind=channel_binding, got %+v", out)
	}
	channels := collectNames(out["channels"])
	topics := collectNames(out["topics"])
	if _, ok := channels["OrderService.placeOrder"]; !ok {
		t.Errorf("expected bound channel OrderService.placeOrder, got channels=%+v", channels)
	}
	if _, ok := topics["kafka:orders.placed"]; !ok {
		t.Errorf("expected bound topic kafka:orders.placed, got topics=%+v", topics)
	}
}

// TestRelated_MessagingDirectionStillDocumentedAndWorks guards that adding
// the neighbors auto-resolution did not regress the explicit
// direction=messaging path.
func TestRelated_MessagingDirectionStillDocumentedAndWorks(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "messaging",
		"entity_id": "kafka:orders.placed",
	})
	if got, _ := out["direction"].(string); got != "messaging" {
		t.Errorf("explicit direction=messaging must echo back direction=messaging, got %q", got)
	}
}

// TestInspect_MessageTopic_IncludesCrossRepoSemanticEdges is fix B's topic
// case: inspecting the topic from the PRODUCER repo must include the
// consumer repo's SUBSCRIBES_TO/DELIVERS_TO, not just the local PUBLISHES_TO.
func TestInspect_MessageTopic_IncludesCrossRepoSemanticEdges(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleGetNode, map[string]any{
		"entity_id": "producer::topic:orders",
	})
	sem, _ := out["semantic_edges"].([]any)
	if len(sem) == 0 {
		t.Fatalf("expected semantic_edges on the topic, got none: %+v", out)
	}
	var haveSub, haveDeliver, havePub bool
	for _, row := range sem {
		m, _ := row.(map[string]any)
		kind, _ := m["kind"].(string)
		switch kind {
		case "SUBSCRIBES_TO":
			haveSub = true
		case "DELIVERS_TO":
			haveDeliver = true
		case "PUBLISHES_TO":
			havePub = true
		}
	}
	if !havePub {
		t.Errorf("expected local PUBLISHES_TO edge, semantic_edges=%v", sem)
	}
	if !haveSub {
		t.Errorf("expected cross-repo SUBSCRIBES_TO edge folded in, semantic_edges=%v", sem)
	}
	if !haveDeliver {
		t.Errorf("expected cross-repo DELIVERS_TO edge folded in, semantic_edges=%v", sem)
	}
}

// TestInspect_ChannelBinding_HasSemanticEdges is fix B's ChannelBinding case:
// inspecting a ChannelBinding must include its BINDS_CHANNEL/BINDS_TOPIC edges.
func TestInspect_ChannelBinding_HasSemanticEdges(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleGetNode, map[string]any{
		"entity_id": "producer::cb:orders-out",
	})
	sem, _ := out["semantic_edges"].([]any)
	if len(sem) == 0 {
		t.Fatalf("expected semantic_edges on the ChannelBinding, got none: %+v", out)
	}
	var haveChannel, haveTopic bool
	for _, row := range sem {
		m, _ := row.(map[string]any)
		switch m["kind"] {
		case "BINDS_CHANNEL":
			haveChannel = true
		case "BINDS_TOPIC":
			haveTopic = true
		}
	}
	if !haveChannel {
		t.Errorf("expected BINDS_CHANNEL edge, semantic_edges=%v", sem)
	}
	if !haveTopic {
		t.Errorf("expected BINDS_TOPIC edge, semantic_edges=%v", sem)
	}
}

// TestBM25_FindsChannelBindingByNaturalLanguage is fix C: a natural-language
// query mentioning the binding's direction/connector/topic (not just the bare
// channel name) must rank the ChannelBinding entity via BM25 — previously
// only search=substring + kind_filter=SCOPE.ChannelBinding could find it.
func TestBM25_FindsChannelBindingByNaturalLanguage(t *testing.T) {
	srv := newMessagingTestServer(t)
	r := srv.State.groups["test"].Repos["producer"]
	idx := r.getBM25()
	if idx == nil {
		t.Fatal("nil BM25 index")
	}
	hits := idx.Search("smallrye kafka outgoing channel binding", 10)
	found := false
	for _, h := range hits {
		if h.Entity != nil && h.Entity.Kind == "SCOPE.ChannelBinding" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected the ChannelBinding entity to be found via bm25 natural-language search, hits=%+v", hits)
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
