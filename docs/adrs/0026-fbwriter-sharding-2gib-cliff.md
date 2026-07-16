# ADR-0026: fbwriter sharding — remove the 2 GiB serialization cliff

- **Status**: **Deferred** (2026-07-17 — measured premise not met; see Outcome). Was: Proposed.
- **Date**: 2026-07-17
- **Deciders**: Jorge Cajas
- **Related**: ADR-0016 (binary graph format), ADR-0017 (single-binary daemon architecture), ADR-0024 (decouple MCP serving from the engine — this is its Phase 1b), ADR-0025 (ChannelBinding)
- **Refs**: #5726 (Fix #2 — remove the cliff; Fix #1 = fail-soft, already merged in Phase 0); convergence #5720 (enrichment / property-vector bloat); read-compat #5775 (kinds serialize by string, not ordinal).

## Outcome — DEFERRED (2026-07-17)

**The motivating premise was measured and does not hold.** This ADR assumed the
monorepo corpus graph "exceeds 2 GiB and currently serializes INCOMPLETE." A
fresh full re-index of the corpus (curantis-monorepos-main: **287,091 entities ·
1,335,957 relationships · 2,001 endpoints**, ~2.1M LOC / ~4.1M raw lines) produced
a **complete** `graph.fb` of **0.404 GiB (434,021,312 bytes) — 20.2 % of the 2 GiB
cliff, ~4.95× headroom.** The write succeeded cleanly (fresh mtime matching the
index; no `cannot grow buffer beyond 2 gigabytes` panic and no fail-soft fallback
in the engine logs). Density is ~325 bytes/relationship, so reaching 2 GiB would
take **~6.6M relationships / ~1.42M entities (~5× the current corpus)** — the ADR's
original "0.5–1M relationships → >2 GiB" estimate over-counted per-record cost
~5–13×, most likely because it predated the relationship-dedup (2.5M→1.4M) and
the #5720 property-vector-bloat convergence.

**Consequences:**
1. **Sharding is deferred.** Building a multi-shard writer/reader/swap/version
   pipeline now would pre-pay for ~5× growth no real corpus has. The single-buffer
   design stands.
2. **The catastrophic failure mode is already handled** by the Phase 0 fail-soft
   guard (`d4c0197e4`, #5726 Fix #1, shipped): an over-cap marshal aborts cleanly
   and preserves the last-good `graph.fb` instead of crashing the daemon.
3. **The EDA research (#20) is NOT blocked by this cliff** — the corpus serializes
   completely, so research can run against the real graph. (Correcting this ADR's
   original "unblocks research" framing: there was nothing to unblock.)
4. **Follow-up instead of sharding:** add a `doctor`/status warning when
   `graph.fb` crosses ~1.5 GiB (75 % of the cap), turning the surprise ceiling
   into a monitored threshold that gives runway to build sharding only if a real
   corpus ever trends toward it. (Filed separately.)

Revisit this ADR only if a real corpus's `graph.fb` approaches ~1.5 GiB. The design
options below are preserved as-is for that eventuality.

## Context

grafel's canonical on-disk artifact is `graph.fb`, a single FlatBuffers buffer
(ADR-0016). The engine writes it on every successful index pass; serve mmaps it
zero-copy and reads FlatBuffers in place (ADR-0024). That single-buffer design
has a hard ceiling: **FlatBuffers' Go builder panics with `cannot grow buffer
beyond 2 gigabytes` in `flatbuffers.(*Builder).growByteBuffer` once the
serialized graph crosses 2 GiB.** On the monorepo corpus (~0.5–1M relationships,
per-property vectors) the marshal *always* crosses that line, so the corpus graph
is currently truncated/blocked and any research over it is invalid.

### Why the "streaming" writer does not help

The name is misleading for this problem. `StreamingWriter`
(`internal/graph/fbwriter/streaming.go:95`) streams entities *into one*
`flatbuffers.Builder` — `sw.b *flatbuffers.Builder` (`streaming.go:96`) — and
only finalizes at `Close()`. `finalize()` (`streaming.go:203`) builds the three
top-level vectors (`GraphStartEntitiesVector` … `EndVector`), closes the `Graph`
root with `FinishGraphBuffer`, and returns `b.FinishedBytes()`. Every entity,
relationship, property entry and community offset lives in that **one** builder
byte-buffer, which starts at 4 MB and grows by doubling (`NewStreamingWriter`,
`streaming.go:115`). It is that single buffer that hits 2 GiB.

`WriteAtomic` → `Marshal` → `streamingMarshal` (`writer.go:37,70`,
`streaming.go:307`) is the *production* write path (callers:
`cmd/grafel/index.go:576`, `internal/extractors/incremental.go:666`,
`internal/dashboard/handlers_enrichment_writeback.go:211`,
`internal/cli/links.go:421`). It takes a fully-assembled `*graph.Document` and
funnels it through the same single builder. So today the corpus write is doubly
memory-bound: the full `*graph.Document` is resident **and** it is copied into a
≤2 GiB builder. `NewStreamingWriter` (the incremental entry point) is currently
only exercised by tests — production does not yet use its per-record model.

### What Phase 0 already did (#5726 Fix #1)

Fail-soft is merged. `finalizeSafe` (`streaming.go:191`) and `streamingMarshal`
(`streaming.go:316`) `recover()` the 2 GiB panic and return
`fbwriter: graph too large to serialize` instead of aborting the daemon.
`WriteAtomic` returns before touching the filesystem on that error
(`writer.go:46`), so the previously-good `graph.fb` stays byte-identical
(`internal/graph/fbwriter/failsoft_5726_test.go`). **Fix #1 keeps the daemon
alive but leaves the corpus graph missing.** This ADR is Fix #2: actually
serialize the whole corpus.

### The load-bearing constraint: zero-copy mmap reads

Any scheme must preserve — or consciously trade off — the read model that ADR-0024
depends on:

- `fbreader.Open` (`reader.go:41`) mmaps the whole file (`mmap_unix.go:45`,
  `unix.Mmap(..., PROT_READ, MAP_SHARED)`) and parses the root with
  `GetRootAsGraph` — **no heap copy**. Fields decode lazily against mmap bytes.
- `LookupEntityByID` (`reader.go:103`) is O(log N) via the FlatBuffers `(key)`
  index on `Entity.id` (`EntitiesByKey`, schema `graph.fbs:24`). This binary
  search is *per-vector* — it only works within one finalized buffer.
- Edge lookups (`IterateRelationshipsFromID`, `reader.go:148`) are O(R) scans of
  the relationship vector; edges reference entities by **string ID**, not by
  vector index (`buildRelationship`, `writer.go:176`). This is what makes
  cross-shard resolution tractable.
- The serve↔engine handoff is a **file swap**: `graph_cache.go` `Get`
  (`graph_cache.go:144`) stats the file, compares `info.ModTime().UnixNano()`
  against the mtime captured at open (`graph_cache.go:63,157`), and transparently
  reopens the mmap when the engine wrote a newer file. A fresh atomic file is the
  *entire* notification protocol (ADR-0024).
- The min-version gate (`internal/graph/load.go:148`, `fbversion.Version` = 4)
  refuses older formats and forces a reindex — grafel is pre-1.0, there is no
  on-disk back-compat contract today.

### Convergence with #5720 (property-vector bloat)

The actual byte driver is the property/enrichment payload. Embedding vectors are
*already* a sidecar: `Entity.embedding_ref` (`graph.fbs:55`) is a content-hash
pointer into `~/.grafel/store/<slug>/embeddings.bin` (`internal/embed/store.go:18`)
rather than an inline vector. #5720 evicts more per-property enrichment bloat the
same way. This **buys headroom but does not remove the cliff**: a large enough
corpus crosses 2 GiB on structural data alone (entity tables + edge tables +
string IDs + property maps). #5720 changes *when* we hit the wall, not *whether*.
See "Open question" — whether structural-only crosses 2 GiB is the pivotal fact.

## Decision drivers

1. The corpus graph must serialize **complete** (research is otherwise invalid).
2. Preserve zero-copy mmap reads and the O(log N) ID lookup.
3. Preserve the atomic, all-or-nothing swap — serve must never see a torn/partial
   graph (ADR-0024 fault isolation + independent deploy).
4. Writer must not need >2 GiB resident to *produce* a shard (bounded RSS).
5. Deterministic, stable-across-rebuild output (bytewise stability, #481 lineage).
6. Minimal, incremental landing — co-exist with the ADR-0024 serve/engine split.

## Options considered

### Option 1 — Multi-segment / sharded files + manifest (RECOMMENDED)

Split the graph into N `graph.<gen>.<k>.fb` shard files, each finalized below a
safe soft cap (default **1.5 GiB**, leaving margin under the 2 GiB hard limit for
the top-level vectors and root table). A small `graph.manifest` (JSON) lists the
shards, their generation, and a routing table. serve mmaps each shard as its own
zero-copy region and presents one logical graph.

- **+** Each shard is an ordinary FlatBuffer → **zero-copy mmap is unchanged**;
  `EntitiesByKey` still works within a shard.
- **+** The manifest is a natural atomic commit point (rename it last) and a
  natural version/format discriminator.
- **+** Writer RSS is bounded: finalize a shard, flush, release its builder,
  start the next.
- **−** Global `LookupEntityByID` needs a routing hop (manifest → shard → binary
  search). Edge scans iterate N segments.
- **−** N mappings per repo (fd / VM-map / LRU pressure).

### Option 2 — One file, N independently-finalized sub-buffers + offset header

Keep a single `graph.fb`, but lay down N finalized FlatBuffers back-to-back with a
fixed header of `(offset, length)` pairs. serve mmaps once and slices the region
into per-sub-buffer views.

- **+** One file, one mmap, one mtime — the existing swap/reload seam is
  *completely* unchanged.
- **+** Still zero-copy; `GetRootAsGraph(buf, offset)` already takes an offset.
- **−** The *file itself* can still exceed what a single 64-bit `int` mmap size
  the current code allows — fine on 64-bit (`mmap_unix.go:41` only rejects the
  32-bit `int` overflow), but a 30 GiB file is one page-cache object and one fd,
  which is operationally coarse (no per-segment eviction, no partial rebuild).
- **−** Writing still needs each sub-buffer's bytes staged before the header is
  known → either two passes or buffering all sub-buffers (RSS regression).
- **−** No independent shard GC; every rewrite is whole-file.

### Option 3 — Structural graph in FlatBuffers + bulky part in a columnar side-segment

Keep entities/edges in `graph.fb`; move the *bulky* payload (property maps /
per-property vectors — the real 2 GiB driver, `buildPropertyVector`,
`writer.go:217`) into a separate columnar side-segment, referenced by an offset
or content hash (exactly how `embedding_ref` already works, `graph.fbs:55`).

- **+** Smallest change to the read model if the structural graph stays < 2 GiB.
- **+** This is the same lever as #5720/embeddings.bin, generalized — arguably the
  *cheapest* fix if structural-only never crosses the cap.
- **−** **Does not remove the cliff in the general case.** If the structural graph
  itself exceeds 2 GiB, Option 3 alone still panics; you then need Option 1 *on
  top*. It is a headroom play, not a guarantee.
- **−** Adds a second on-disk format (columnar) to maintain and version.

## Decision

Adopt **Option 1 (sharded files + manifest)** as the durable, guarantee-level
fix, and treat **Option 3 (side-segment / #5720)** as the complementary
size-reduction that *lowers shard count* but is not relied on for correctness.
Option 2 is rejected: it preserves the swap seam but regresses writer RSS and
gives up per-shard eviction and partial rebuild, which the ADR-0024 split wants.

Rationale: Option 1 is the only candidate that (a) hard-guarantees no single
buffer approaches 2 GiB regardless of corpus growth, (b) keeps every shard a
plain zero-copy FlatBuffer so `EntitiesByKey` and lazy field decode are unchanged,
and (c) makes the manifest a clean atomic-commit + format-discriminator that
extends — rather than replaces — the existing mtime-reload seam.

### Sharding key + boundary policy

Two independent streams, two policies:

- **Relationships** are sharded by a **byte budget**. Edges are only ever *scanned*
  (`IterateRelationshipsFromID/ToID`, `reader.go:148,166`), never keyed, so their
  physical partition is irrelevant to correctness. Fill relationship shard *k* in
  emission order until the running serialized-size estimate approaches the soft
  cap, then roll to *k+1*. Simple, deterministic, byte-bounded.

- **Entities** are sharded by **byte-budget sequential fill that preserves the
  existing sort order**. The pipeline already runs `sortDocumentForEmission`
  before writing (`streaming.go:207`), so entities arrive sorted by ID. Fill
  entity shard *k* until the budget is hit, then start *k+1*. Because the split
  points fall on the *sorted* stream, each shard owns a contiguous, disjoint ID
  range `[minID, maxID]`. The manifest records that range per shard.

  This yields a **routing table**: `LookupEntityByID(id)` binary-searches the
  manifest range table (O(log S), S = shard count, tiny) to pick the owning
  shard, then calls that shard's `EntitiesByKey` (O(log n)). Total O(log S + log n)
  ≈ O(log N) — the existing complexity, plus one cheap hop. Determinism is
  preserved: same sorted input ⇒ same split points ⇒ same shard boundaries across
  rebuilds (modulo the soft-cap knob).

  Rejected alternative — **hash-partition** (`shard = hash(id) % N`): O(1) shard
  selection and no routing table, but it *cannot* honor a byte budget (a hot
  bucket may exceed the cap) and it destroys the sorted layout the key index and
  bytewise stability rely on. The range policy keeps both.

**Cross-shard references resolve by ID, never by index.** Edges already carry
string `from_id`/`to_id` (`writer.go:176`); a caller resolving an edge target
calls the unified `LookupEntityByID`, which routes to whatever shard owns that ID.
No edge needs to know which shard its endpoints live in.

### Reader unified view

Introduce a `MultiReader` (or promote `Reader` to hold `[]*mmapRegion` +
`[]*fb.Graph` + the manifest routing table) that presents the current `Reader`
API unchanged:

- `Open(dir)` reads `graph.manifest`, mmaps each listed shard (each its own
  `mmapRegion`, `mmap_unix.go:23`), and parses each shard's `Graph` root. **Zero-copy
  is preserved per shard** — no concatenation, no heap copy.
- `EntityCount` = Σ per-shard counts; `RelationshipCount` = Σ.
- `LookupEntityByID` routes via the range table, then per-shard `EntitiesByKey`.
- `IterateEntities` / `IterateRelationships` (`reader.go:307,330`) iterate shards
  in order — same visitor contract, N segments instead of one.
- `IterateRelationshipsFromID/ToID` scan all relationship shards (same O(R) total).
- `LoadGraphMeta` / `LoadAlgoStats` / communities: **graph-level metadata and the
  Pass-4 aggregates live in shard 0's `Graph` root** (or, cleaner, in the
  manifest). Only shard 0 carries the header; other shards carry empty
  header + populated vectors. The manifest is authoritative for version and
  shard set.

Lifetime: the Reader pins all N mmaps until `Close`; the graph_cache
`Entry.refs` refcount (`graph_cache.go:64`) gates teardown exactly as today, now
over a shard set rather than one file.

### Atomic multi-shard swap — the manifest is the commit point

Extend the current write-tmp+rename+mtime-reload (`streaming.go:178`,
`graph_cache.go:157`) to a **manifest-pointer** protocol:

1. Engine writes each shard to a **generation-suffixed, immutable** name:
   `graph.<gen>.<k>.fb` (gen = monotonic counter or index timestamp). Because
   names are generation-unique, a new write **never overwrites** a shard an old
   reader is still mmap'ing — this is what makes the swap safe with multiple
   segments.
2. `fsync` each shard.
3. Write `graph.manifest.tmp` (listing gen, shard filenames, per-shard ID range,
   entity/rel counts, version), `fsync` it, then **`rename` it onto
   `graph.manifest` last**. The rename is the single atomic commit — before it,
   serve sees the old manifest → old shards; after it, the new set.
4. serve keys reload on **`graph.manifest`'s mtime** (replacing the `graph.fb`
   mtime check at `graph_cache.go:157`). One stat, one atomic pointer.
5. GC: after the manifest flips, older-generation shard files are removed **only
   once no cached Reader references them** (drain via the existing `refs`
   refcount, `graph_cache.go:335`). A crash between rename and GC just leaves
   orphan shard files (harmless, swept next pass).

No partial/torn read is possible: serve either sees generation N's manifest and
all of N's shards (immutable, already fsync'd) or generation N-1's. There is no
in-between.

### Format-version bump + read-compat

- Bump `fbversion.Version` 4 → **5**. Presence of `graph.manifest` is the format
  discriminator; a bare `graph.fb` with no manifest = legacy single-file.
- **Always emit a manifest**, even for a 1-shard graph, so serve has exactly one
  code path. A small graph produces `graph.manifest` + `graph.0.fb` — the common
  case is still a single mmap, just reached via the manifest.
- **serve reads both** during rollout: manifest present → multi-shard path; else
  legacy `graph.fb` (`load.go:44` fallback stays). This matters because ADR-0024
  deploys serve and engine independently. **Rollout ordering is mandatory: ship
  the serve reader that understands manifests BEFORE the engine starts writing
  them** — otherwise an upgraded engine writes a shard set an old serve cannot
  read. See the risk register and open question.
- Migration of existing on-disk graphs: **rebuild-on-upgrade**, consistent with
  the existing min-version gate (`load.go:148`) that already forces a reindex on a
  version bump. No in-place converter. Under-cap repos rebuild to a 1-shard
  manifest transparently on their next index pass.

### Interaction with the ADR-0024 split

Roles are unchanged and sharpened: **engine writes** shards + manifest atomically
(bounded per-shard RSS); **serve mmaps** the shard set and mtime-reloads on the
manifest. Fault isolation holds — a mid-write engine crash leaves the previous
manifest pointing at the previous complete generation, and serve never observes a
partial set. Independent deploy holds *provided* the reader-first ordering above
is respected. This is literally the "Phase 1b shards the fbwriter" item named in
ADR-0024's status line.

### Memory bounds (writer)

The corpus write MUST go through the incremental model, not `WriteAtomic(fullDoc)`:

- Route the full-index / corpus write through a **sharding StreamingWriter** fed
  entity-by-entity and relationship-by-relationship (the `NewStreamingWriter`
  model that is currently test-only).
- Maintain a running **serialized-size estimate** as offsets accumulate. When it
  approaches the soft cap, `finalize()` the current shard, write+fsync it, **drop
  the builder reference** so GC reclaims up to ~1.5 GiB, and allocate a fresh
  `flatbuffers.NewBuilder` for the next shard.
- Peak writer working set ≈ one shard's builder (≤1.5 GiB) + the small offset
  slices, instead of full-doc + a ≤2 GiB builder. This also removes the current
  double-buffering (`*graph.Document` resident *and* copied into the builder) on
  the corpus path.

Illustrative rollover (design sketch, not final code):

```go
// pseudo-code — inside the sharded streaming writer
func (w *ShardWriter) WriteEntity(e *graph.Entity) error {
    if w.cur.estBytes()+estimate(e) > w.softCap { // e.g. 1.5 GiB
        if err := w.flushShard(); err != nil {     // finalize+fsync+record range
            return err
        }
        w.startShard()                             // fresh Builder, GC reclaims old
    }
    return w.cur.WriteEntity(e)                     // existing streaming.go path
}
```

## Consequences

### Positive

- The corpus graph serializes **complete** — the EDA research is unblocked.
- No single buffer ever approaches 2 GiB; the cliff is *structurally* gone, not
  merely fail-softed.
- Writer RSS is bounded to one shard, removing the corpus double-buffer.
- Manifest gives per-shard eviction and partial rebuild affordances for the future.
- Read model, lookup complexity, and the atomic-swap seam are preserved.

### Negative / costs

- Two new artifacts (manifest + shard-naming) and a routing table to maintain.
- Extra indirection on the hottest lookup (`LookupEntityByID` gains one O(log S)
  hop) and multi-segment iteration on edge scans.
- More open fds / mmaps per repo; the graph_cache LRU (`DefaultCapacity` = 10,
  `graph_cache.go:35`) counts *files* today and will count *shards* — needs
  re-tuning or a per-repo (not per-file) accounting.
- A version bump forces a corpus reindex on upgrade.

## Phased effort & critical path

| Phase | Scope | Rough size | Lands incrementally? |
|---|---|---|---|
| **A. Writer sharding** | Route corpus write through streaming; size-estimate + rollover; per-shard finalize/fsync/release; emit manifest (incl. 1-shard case) | **M–L** | Manifest format + 1-shard emission can land first (byte-compatible with today) |
| **B. Reader multi-segment** | `MultiReader` over N mmaps + manifest routing; `EntitiesByKey` routing; iterate all shards; meta from shard0/manifest | **M** | Must land (and deploy to serve) **before** A emits >1 shard |
| **C. Atomic swap + manifest reload** | Generation-suffixed immutable shard names; manifest rename as commit; graph_cache reloads on manifest mtime; generation GC via refcount | **S–M** | Co-dependent with B for serve |
| **D. Version bump + read-compat** | `fbversion` 4→5; manifest-vs-legacy discriminator; serve reads both; rebuild-on-upgrade | **S** | Yes |
| **E. Tests** | Lowered shard-cap knob; multi-shard on a tiny graph; torn-read/concurrent-swap; cross-shard lookup parity; migration | **M** | Continuous |

**Critical path: A → B → C**, gated by the **B-before-A deploy ordering** (serve
must read manifests before engine writes them). D and E parallelize.

**Incremental landing plan:** (1) manifest + always-1-shard writer (no behavior
change, byte-compatible bridge); (2) `MultiReader` that transparently handles the
1-shard manifest; (3) deploy the reader to serve; (4) flip on multi-shard rollover
in the writer; (5) generation GC + LRU re-tuning.

## Risk register

| Risk | Severity | Mitigation |
|---|---|---|
| **Torn / partial read** — serve sees manifest gen N but a shard is half-written or already GC'd | High | Immutable generation-suffixed shard names; fsync shards before manifest; manifest rename is the only commit; GC only after `refs`==0 |
| **Cross-shard reference bug** — routing table wrong → dangling/missing edges, wrong `find_callers` | High | Property-based parity test: same graph, single-file vs sharded, must return identical lookups/traversals; assert range-table coverage is total + disjoint |
| **Deploy ordering** — upgraded engine writes shards an old serve can't read (ADR-0024 independent deploy) | High | Ship reader-understands-manifest first; serve reads both formats; document the ordering as a release gate |
| **mmap fd / VM-map limit & RSS** — N shards × R repos multiplies mappings; LRU counts files not repos | Med | Re-tune graph_cache to account per-repo shard sets; raise `RLIMIT_NOFILE`; cap shard count via larger soft cap |
| **Hot-lookup perf regression** — extra routing hop + multi-segment edge scans | Med | Benchmark `LookupEntityByID` / `IterateRelationships*`; keep S small (1.5 GiB cap ⇒ few shards); range table stays in-process |
| **Migration of existing graphs** | Low | Rebuild-on-upgrade via existing min-version gate; no in-place converter |
| **Bytewise-stability regression** (#481) | Med | Sequential (sorted) fill keeps deterministic split points; snapshot-test shard boundaries for a fixed corpus + fixed cap |

## Test strategy — exercising >2 GiB without a 2 GiB fixture

The core trick: make the **soft cap a knob** (env var / config, e.g.
`GRAFEL_FB_SHARD_SOFT_CAP_BYTES`). Set it to a few KB in tests so a tiny,
fast fixture graph produces many shards and exercises every multi-shard path —
rollover, manifest, routing, cross-shard lookup, atomic swap — with zero large
allocation. Concretely:

- **Multi-shard parity:** build a small graph, write it single-file and sharded
  (tiny cap), assert `LookupEntityByID`, kind filters, and edge traversals return
  identical results.
- **Boundary correctness:** force split points *between* an edge's two endpoints
  so the endpoints land in different shards; assert resolution still works.
- **Atomic swap / torn read:** concurrent reader loop + writer generation flip;
  assert the reader only ever observes a complete generation (never a mixed set).
- **Deterministic boundaries:** fixed corpus + fixed cap ⇒ identical shard byte
  boundaries across runs (bytewise stability).
- **Real 2 GiB (nightly/optional):** retain a gated large-synthetic test, plus the
  existing `marshalPanicHook` seam (`streaming.go:305`) to assert the *old*
  single-buffer path still fail-softs — the Fix #1 safety net must survive.

## Relationship to #5726 Fix #1 and #5720 — sequencing

- **#5726 Fix #1 (fail-soft, merged, Phase 0):** stays as the **safety net**. Even
  after sharding, `finalizeSafe` guards each shard's finalize; if a single shard
  somehow still overflows (mis-tuned cap, pathological entity), it fail-softs
  rather than crashes. Fix #2 (this ADR) removes the *cause*; Fix #1 remains the
  belt-and-braces.
- **#5720 (property/enrichment bloat):** land **first** — it is a cheap size
  reduction (evict bloat to sidecars like `embeddings.bin`) that lowers shard
  count and may keep many repos at a single shard. But it is **not** relied on for
  correctness; the sharding guarantee stands on its own.
- **Recommended order:** #5720 (shrink) → Phase A/B/C sharding (guarantee) →
  retire the corpus's dependence on Fix #1 for the happy path.

## Open question for the maintainer

**Does the corpus's *structural* graph — entities + edges + string IDs + property
maps, with all vectors/enrichment blobs already evicted to sidecars (#5720 +
`embeddings.bin`) — realistically exceed 2 GiB?**

This is the pivotal fork:

- **If NO:** Option 3 (push the last bulky payload to a columnar side-segment)
  keeps `graph.fb` a single sub-2 GiB FlatBuffer and is *dramatically* cheaper —
  no routing table, no multi-segment reader, no manifest swap. We ship #5720 hard
  and defer general sharding.
- **If YES:** general sharding (Option 1) is **mandatory** — no amount of blob
  eviction saves you — and we build A→B→C now.

I recommend Option 1 because it is the only correctness *guarantee* against
unbounded corpus growth, but the effort delta between "ship #5720 + a side-segment"
and "build full sharding" is large enough that the maintainer should decide based
on a single measured number: **the serialized size of the corpus with all vectors
stripped.** If that number is comfortably under ~1.5 GiB with headroom for growth,
we can sequence sharding behind #5720 rather than ahead of it.
