package mcp

// channelbinding_di_test.go — #5782 (ADR-0025) semantic-bleed guard: a
// ChannelBinding's channel edge (BINDS_CHANNEL) must never be projected as a
// dependency-injection edge in grafel_inspect / neighbors output.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

func TestIsDIEdgeKind_ExcludesBindsChannel(t *testing.T) {
	// The config→channel edge kind must NOT be treated as a DI edge.
	if isDIEdgeKind(string(types.RelationshipKindBindsChannel)) {
		t.Error("BINDS_CHANNEL must NOT be classified as a dependency-injection edge")
	}
	// Contrast: the reused Helm/DI "BINDS" kind IS a DI edge — the reason we
	// use a distinct BINDS_CHANNEL kind for messaging bindings.
	if !isDIEdgeKind(string(types.RelationshipKindBinds)) {
		t.Error("BINDS should still be classified as a DI edge (contrast case)")
	}
	// BINDS_TOPIC is likewise not a DI edge.
	if isDIEdgeKind(string(types.RelationshipKindBindsTopic)) {
		t.Error("BINDS_TOPIC must NOT be classified as a dependency-injection edge")
	}
}
