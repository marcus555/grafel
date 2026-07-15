package indexstate

import "testing"

// TestIsEnhancingDistinctFromIndexing proves the Snapshot distinguishes
// EXTRACTION (index jobs in flight → IsIndexing) from the ENRICHMENT tail (a
// group-algorithm pass in flight → IsEnhancing). A pure group-algo pass must
// NOT report as "indexing" — the enrichment tail is enhancing, not indexing.
func TestIsEnhancingDistinctFromIndexing(t *testing.T) {
	t.Cleanup(func() { Set(0); GroupAlgoEnd() })

	Set(0)
	if s := Get(); s.IsIndexing || s.IsEnhancing {
		t.Fatalf("idle: got %+v, want neither indexing nor enhancing", s)
	}

	// Index job in flight → indexing, not enhancing.
	Set(1)
	if s := Get(); !s.IsIndexing || s.IsEnhancing {
		t.Fatalf("index job: got %+v, want indexing=true enhancing=false", s)
	}
	Set(0)

	// Pure group-algo enrichment pass → enhancing, NOT indexing.
	GroupAlgoBegin()
	if s := Get(); s.IsIndexing || !s.IsEnhancing {
		t.Fatalf("group-algo: got %+v, want indexing=false enhancing=true", s)
	}
	if !Get().Busy {
		t.Fatal("group-algo: Busy must remain true during enrichment")
	}
	GroupAlgoEnd()

	if s := Get(); s.IsIndexing || s.IsEnhancing {
		t.Fatalf("after end: got %+v, want neither", s)
	}
}
