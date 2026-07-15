package repolock

import (
	"testing"
	"time"
)

// TestForegroundClaimStart exposes the claim's start timestamp so the status
// writer can distinguish "graph not yet written this run" (extraction →
// indexing) from "graph written this run, still claimed" (enrichment →
// enhancing) by comparing graph.fb mtime against the claim start.
func TestForegroundClaimStart(t *testing.T) {
	r := New()
	const key = "/repo/x"

	if start, held := r.ForegroundClaimStart(key); held || start != 0 {
		t.Fatalf("before claim: got (start=%d, held=%v), want (0, false)", start, held)
	}

	before := time.Now().UnixNano()
	rel := r.ClaimForeground(key)
	after := time.Now().UnixNano()

	start, held := r.ForegroundClaimStart(key)
	if !held {
		t.Fatal("during claim: held should be true")
	}
	if start < before || start > after {
		t.Fatalf("claim start %d not within [%d,%d]", start, before, after)
	}

	rel()
	if start, held := r.ForegroundClaimStart(key); held || start != 0 {
		t.Fatalf("after release: got (start=%d, held=%v), want (0, false)", start, held)
	}
}
