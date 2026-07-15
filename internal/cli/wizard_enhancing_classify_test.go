package cli

// wizard_enhancing_classify_test.go — TDD coverage for the false-"Failed" the
// wizard reports when the rebuild RPC acks while the long background ENRICHMENT
// tail is still running. The status plane now splits the single `indexing` flag
// into `indexing` (extraction, graph not yet queryable) and `enhancing`
// (graph queryable, enrichment tail running). A repo that is queryable —
// graph.fb (re)written past the baseline — must classify as SUCCESS even while
// it is still enhancing, and even if a stale Indexing flag is still set at the
// instant the rebuild acked.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/statusfile"
)

// A queryable repo that is still ENHANCING (indexing=false, enhancing=true,
// graph.fb advanced past baseline) must classify as indexed-OK, never Failed.
func TestStatusProbe_ClassifiesOKWhileEnhancing(t *testing.T) {
	const repo = "/repo/enhancing"
	store := newFakeStatusStore() // fresh group: baseline 0
	probe := newStatusPlaneProbeWith([]string{repo}, "/root", store.read, aliveLiveness, pendingUntil(1))

	// Extraction done → graph queryable, but the enrichment tail is still running.
	store.set(repo, &statusfile.File{Indexing: false, Enhancing: true, GraphFBMtime: 2_000_000, Entities: 42, Relationships: 7})

	res, _ := probe.Classify()
	if len(res.Failed) != 0 {
		t.Fatalf("Classify marked %v as failed while queryable+enhancing (false failure)", res.Failed)
	}
	if res.Entities != 42 || res.Rels != 7 {
		t.Fatalf("stats = (%d,%d); want (42,7)", res.Entities, res.Rels)
	}
}

// The exact live symptom: the rebuild RPC acks while enrichment is still
// running, so the engine's status flush still shows a stale Indexing=true —
// yet the graph is queryable (mtime advanced) and the repo IS enhancing. This
// must classify as SUCCESS, NOT "still indexing when the rebuild acked".
func TestStatusProbe_ClassifiesOKWhenAckedMidEnhance(t *testing.T) {
	const repo = "/repo/acked-mid-enhance"
	store := newFakeStatusStore()
	probe := newStatusPlaneProbeWith([]string{repo}, "/root", store.read, aliveLiveness, pendingUntil(1))

	// Queryable (graph written) AND enhancing, but a stale Indexing flag is still
	// set at ack time. The old classifier fails this with "still indexing when
	// the rebuild acked"; the enhancing-aware classifier must treat it as OK.
	store.set(repo, &statusfile.File{Indexing: true, Enhancing: true, GraphFBMtime: 3_000_000, Entities: 287_000, Relationships: 900_000})

	res, _ := probe.Classify()
	if len(res.Failed) != 0 {
		t.Fatalf("Classify marked %v as failed on a queryable+enhancing repo whose rebuild acked mid-enhance (the live false-Failed)", res.Failed)
	}
	if res.Entities != 287_000 {
		t.Fatalf("Entities = %d; want 287000 counted for the queryable repo", res.Entities)
	}
}
