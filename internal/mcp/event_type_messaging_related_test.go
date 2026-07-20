// Tests for the GAP-005 load-bearing MCP change: isMessageTopicEntity /
// resolveTopicSeed (messaging_related_5782.go) extended to also seed on
// SCOPE.EventType (+ SCOPE.EventBusEvent freebie), so a producer
// PUBLISHES_TO / consumer SUBSCRIBES_TO join through a shared
// event-identity node is reachable via the SAME grafel_related
// direction=messaging / direction=neighbors query surface as a
// SCOPE.MessageTopic.
package mcp

import (
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// newEventTypeTestServer builds a single-repo group whose producer and
// consumer are joined through ONE SCOPE.EventType node — exactly the shape
// event_type_edges.go's engine pass emits (GAP-005 keystone):
//
//	PublishOrderPlaced        --PUBLISHES_TO-->   event:type:OrderPlaced
//	OrderConsumer.onOrderPlaced --SUBSCRIBES_TO--> event:type:OrderPlaced
func newEventTypeTestServer(t *testing.T) *Server {
	t.Helper()

	const eventTypeName = "event:type:OrderPlaced"

	doc := minDoc(
		[]graph.Entity{
			{ID: "fn:publish", Name: "PublishOrderPlaced", Kind: "SCOPE.Function", SourceFile: "producer.go", StartLine: 10},
			{ID: "fn:consume", Name: "OrderConsumer.onOrderPlaced", Kind: "SCOPE.Function", SourceFile: "consumer.go", StartLine: 20},
			{ID: "et:orderplaced", Name: eventTypeName, Kind: "SCOPE.EventType", SourceFile: ""},
		},
		[]graph.Relationship{
			{ID: "pub:1", FromID: "fn:publish", ToID: "et:orderplaced", Kind: "PUBLISHES_TO"},
			{ID: "sub:1", FromID: "fn:consume", ToID: "et:orderplaced", Kind: "SUBSCRIBES_TO"},
		},
	)
	doc.Repo = "orders-service"

	reg := &Registry{Groups: map[string]RegistryGroup{
		"test": {Repos: map[string]RegistryRepo{
			"orders-service": {Path: t.TempDir()},
		}},
	}}
	st := NewState(reg)
	st.mu.Lock()
	lg := &LoadedGroup{
		Name: "test",
		Repos: map[string]*LoadedRepo{
			"orders-service": {Repo: "orders-service", Doc: doc, LabelIndex: BuildLabelIndex(doc)},
		},
	}
	st.groups["test"] = lg
	st.mu.Unlock()

	return &Server{State: st, Tel: NewTelemetry(0)}
}

// TestEventType_MessagingRelated_SeedResolvesByName is the keystone
// assertion: grafel_related direction=messaging on the VERBATIM event-type
// string reaches BOTH the producer and the consumer through the shared
// SCOPE.EventType node.
func TestEventType_MessagingRelated_SeedResolvesByName(t *testing.T) {
	srv := newEventTypeTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "messaging",
		"entity_id": "event:type:OrderPlaced",
	})
	if out == nil {
		t.Fatal("nil/non-JSON result")
	}
	if resolved, _ := out["resolved"].(bool); !resolved {
		t.Fatalf("expected resolved=true seeding on a SCOPE.EventType node, got %+v", out)
	}

	producers := collectNames(out["producers"])
	consumers := collectNames(out["consumers"])

	if repo := producers["PublishOrderPlaced"]; repo != "orders-service" {
		t.Errorf("expected producer PublishOrderPlaced reachable from the EventType seed, got producers=%+v", producers)
	}
	if repo := consumers["OrderConsumer.onOrderPlaced"]; repo != "orders-service" {
		t.Errorf("expected consumer OrderConsumer.onOrderPlaced reachable from the EventType seed, got consumers=%+v", consumers)
	}
}

// TestEventType_MessagingRelated_DefaultNeighborsDirection verifies the
// direction=neighbors (the tool's own default) fallback also seeds on
// SCOPE.EventType via tryMessagingNeighbors, mirroring the existing
// SCOPE.MessageTopic behavior (#5782 ask #3).
func TestEventType_MessagingRelated_DefaultNeighborsDirection(t *testing.T) {
	srv := newEventTypeTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "neighbors",
		"entity_id": "orders-service::et:orderplaced",
	})
	if out == nil {
		t.Fatal("nil/non-JSON result")
	}
	consumers := collectNames(out["consumers"])
	if _, ok := consumers["OrderConsumer.onOrderPlaced"]; !ok {
		t.Errorf("expected default-direction neighbors to seed on SCOPE.EventType, got consumers=%+v", consumers)
	}
}

// TestEventType_MessagingRelated_DoesNotRegressMessageTopic guards that
// extending isMessageTopicEntity to SCOPE.EventType did not break the
// pre-existing SCOPE.MessageTopic seeding path.
func TestEventType_MessagingRelated_DoesNotRegressMessageTopic(t *testing.T) {
	srv := newMessagingTestServer(t)
	out := callFlowTool(t, srv.handleCoreRelated, map[string]any{
		"direction": "messaging",
		"entity_id": "kafka:orders.placed",
	})
	if out == nil {
		t.Fatal("nil/non-JSON result")
	}
	if resolved, _ := out["resolved"].(bool); !resolved {
		t.Fatalf("expected SCOPE.MessageTopic seeding to still resolve, got %+v", out)
	}
}
