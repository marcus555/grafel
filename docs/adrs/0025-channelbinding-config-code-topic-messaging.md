# ADR-0025: ChannelBinding — connect config↔code↔topic for messaging (reference impl: Quarkus/SmallRye/Kafka)

- **Status**: Proposed
- **Issue**: #5782 (Phase 2/3). Builds on #5781 (`SCOPE.MessageTopic` kind recognition in topology/find).
- **Ships in**: v0.1.8.1 (Quarkus/SmallRye/Kafka reference slice); generalization is backlog.

## Context

grafel models three messaging facts in three disconnected places today:

1. **Config** — `internal/extractors/config/discover.go` emits exactly one
   `SCOPE.Config` entity per recognised file (`buildConfigEntity`,
   discover.go:348; `Discover`, discover.go:259). Properties keys are parsed by
   format: `.properties` files go through `parseProperties`
   (discover.go:877), which collapses every key to a sorted `keys_top_level`
   string — the values are discarded. Subtypes `spring_properties` /
   `quarkus_properties` are assigned in the `classify` table (discover.go:118).
   The only edges emitted are `DEPENDS_ON_CONFIG` (module→config,
   discover.go:299) and `CONFIGURES` (config→module, discover.go:310). So
   `mp.messaging.incoming.<ch>.connector` / `.topic` / `.serializer` exist only
   as discarded flat property keys — the Kafka **topic name is never captured
   as graph signal**.

2. **Code channels** — `internal/custom/java/microprofile.go`
   (`ExtractMicroProfile`, microprofile.go:52) recognises `@Incoming("<ch>")`
   and `@Outgoing("<ch>")` (`mpIncomingRE`/`mpOutgoingRE`, microprofile.go:32/36)
   and emits `SCOPE.Operation` entities carrying `channel` + `direction`
   properties (microprofile.go:158-185). Kotlin equivalent:
   `internal/custom/kotlin/micronaut_quarkus.go`. The code entity carries the
   **channel** name; it never sees the topic.

3. **Topics** — the engine synthesizes `SCOPE.MessageTopic` entities with a
   broker-prefixed `Name` (`"kafka:<topic>"`), joined cross-repo by
   `internal/links/topic_pass.go` P7 (`MethodTopic`, topic_pass.go:46;
   `isTopicEntityKind`, topic_pass.go:72) into `links.json`.

The channel (code) and the topic (config) are two names for the same wire, but
`mp.messaging.<direction>.<channel>.topic=<topic>` — the row that maps one to
the other — lives only in the discarded config bag. The graph therefore cannot
answer "which topic does this `@Outgoing` method publish to?" or "is this
Kafka topic consumed by anyone?", and effect analysis
(`internal/substrate/effects.go`) has **no message-publish element at all**
(lattice is `db_read/write, http_out, fs_read/write, mutation, env_read` —
effects.go:33-59; `AllEffects`, effects.go:65).

## Decision

Introduce a first-class **`SCOPE.ChannelBinding`** entity that captures the
config row mapping channel↔connector↔topic, and wire it to both the code
`SCOPE.Operation` (by channel) and the engine `SCOPE.MessageTopic` (by topic).
Separately, add a **`message_publish`** effect-lattice element. Ship both for
Quarkus/SmallRye/Kafka as the reference implementation, behind a
framework-agnostic seam that the generalization backlog extends.

---

## 1. ChannelBinding entity model

### 1.1 Entity

New kind `EntityKindChannelBinding = "SCOPE.ChannelBinding"` registered in
`internal/types/kinds.go` (const block near kinds.go:76, and appended to
`AllEntityKinds()` — kinds.go:339). One entity per
`mp.messaging.{incoming,outgoing}.<channel>` group.

| Property | Source key | Example |
|---|---|---|
| `channel` | the `<ch>` path segment | `orders-out` |
| `direction` | `incoming` / `outgoing` | `outgoing` |
| `connector` | `…​.<ch>.connector` | `smallrye-kafka` |
| `topic` | `…​.<ch>.topic` (falls back to `channel` per SmallRye default) | `orders.placed` |
| `serializer` | `…​.<ch>.{value.serializer,serializer}` | `…KafkaJsonSerializer` |
| `source_config` | rel path of the emitting config file | `src/main/resources/application.properties` |

- `SourceFile` = the config file; `StartLine`/`EndLine` = 1 (config entities are
  single-point, matching `buildConfigEntity`).
- ID: `scope:channelbinding:<subtype>:<relpath>:<direction>:<channel>` (stable,
  mirrors the `scope:config:` scheme at discover.go:378).

### 1.2 Hook function — new recognizer in config/discover.go

`parseProperties` (discover.go:877) currently only collects key names. Add a
sibling recognizer that runs **inside `Discover`** (discover.go:284, right after
`buildConfigEntity`, before edge emission), gated to the messaging-capable
subtypes:

```
func discoverChannelBindings(repoRoot, rel string, spec configSpec, content []byte)
        ([]types.EntityRecord, []types.RelationshipRecord)
```

- Gate: `spec.subtype == "quarkus_properties" || spec.subtype == "spring_properties"`
  (the reference slice; extended per §5). Also handle YAML variants
  (`application.yaml`) via the existing `parseYAML` path — SmallRye accepts both.
- Regex over raw content (NOT the collapsed `keys_top_level` — we need values):
  `mp\.messaging\.(incoming|outgoing)\.([^.]+)\.(connector|topic|value\.serializer|serializer)\s*=\s*(.+)`
  Group by `(direction, channel)` into one ChannelBinding each.
- **Value capture is a deliberate departure** from the value-discard posture of
  `parseProperties`/`parseEnv` (discover.go:917 — env values are security-
  sensitive and dropped). Connector/topic/serializer are non-secret structural
  identifiers; capturing them is safe and is the whole point. Document this
  exception inline.
- Return entities + edges so `Discover` appends them to its slices before
  `sortEntities`/`sortRels` (discover.go:320). Deterministic sort already covers
  the new records.

### 1.3 Edges

Emitted by `discoverChannelBindings` as unresolved structural refs; the
intra-repo resolver (`internal/resolve/refs.go`) rebinds them:

- **`BINDS` : ChannelBinding → SCOPE.Operation**, matched by `channel` within
  the repo. Target ref key: the `channel` value. Resolution joins against the
  `channel` property that `ExtractMicroProfile` already stamps on the Operation
  (microprofile.go:167/181). Direction must agree (incoming binding →
  `@Incoming` op; outgoing → `@Outgoing`).
- **`BINDS_TOPIC` : ChannelBinding → SCOPE.MessageTopic**, matched by
  `Name == "kafka:" + topic`. This is the join the graph never had — it makes
  the code operation reachable to the topic via
  `Operation ←BINDS— ChannelBinding —BINDS_TOPIC→ MessageTopic`, and thus
  cross-repo through P7's existing topic join (topic_pass.go).

New relationship kinds go in `internal/types` alongside `DependsOnConfig`/
`Configures` (used at discover.go:302/313).

### 1.4 Producer-less / consumer-less detection

After resolution, a validation pass (co-located with the topology channel
scans touched by #5781, `internal/dashboard/handlers_topology.go`) flags:

- **Orphan outgoing binding**: `direction=outgoing`, has `topic`, but `BINDS`
  resolved to no `@Outgoing` Operation → config declares a producer channel no
  code publishes to.
- **Orphan incoming binding**: symmetric, no `@Incoming` consumer.
- **Dangling topic**: `BINDS_TOPIC` unresolved → config names a topic the
  engine never synthesized a `SCOPE.MessageTopic` for (typo / external topic).

Surface these through the existing topology orphan-publisher/subscriber scans
that #5781 already routes `SCOPE.MessageTopic` into — the ChannelBinding gives
those scans the config-side ground truth they lacked.

---

## 2. #4 — message-publish effect

### 2.1 Lattice element

`internal/substrate/effects.go`:

- Add `EffectMessagePublish Effect = "message_publish"` to the const block
  (effects.go:33-59).
- Append it to `AllEffects()` (effects.go:65). **Order is load-bearing** — the
  comment at effects.go:62 warns that `EffectSet` packs effects into bit
  positions by this order. Append at the **end** (position 7) to avoid
  renumbering existing sidecars.
- Bump `effectSlots` 7 → 8 (effects.go:75). This constant must equal
  `len(AllEffects())` — there is a test guarding it.

### 2.2 Sniffer additions

Effect sniffers are per-language, registered via `RegisterEffectSniffer`
(effects.go:116). Add a message-publish regex to each:

- **Java** — `internal/substrate/effect_sinks_java.go`. `sniffEffectsJava`
  (effect_sinks_java.go:92) currently appends HTTP/DB/FS/mutation matches
  (lines 98-103). Add:
  - `Emitter.send(...)` / `MutinyEmitter.send` / `.sendMessage(` — SmallRye
    reactive-messaging emitter publish.
  - Presence of an `@Outgoing("…")` method body → the method itself is a
    publisher (return-value publish, no explicit `.send`).
  - New line: `appendJavaMatches(out, content, headers, javaMsgPublishRe,
    EffectMessagePublish, "smallrye.Emitter.send/@Outgoing", 0.9)`.
- **Kotlin** — `internal/substrate/effect_sinks_kotlin.go`
  (`sniffEffectsKotlin`, effect_sinks_kotlin.go:100, registered
  effect_sinks_kotlin.go:35). Same `Emitter.send` / `@Outgoing` pattern via
  `appendKotlinMatches`.

### 2.3 Re-index requirement + downstream ripple

Effects are **pre-baked during indexing** and stamped as properties on
Function/Operation entities (effects.go:21-27) — they are not computed at query
time. Therefore a new lattice element requires a **full re-index** of any group
that wants message_publish signal (see §6).

Ripple:
- **Effects sidecar** — `internal/links/effect_propagation.go` propagates the
  effect set along the call graph and writes the sidecar; the 8th bit flows
  through automatically once `effectSlots` is bumped.
- **Effects tool** — `internal/mcp/effects_tool.go` (`handleEffects`,
  effects_tool.go:108; `loadEffectsSidecar`, effects_tool.go:85) returns the
  `Effects []string` list (effects_tool.go:51) verbatim; `message_publish`
  appears with no code change, but the tool's help/description text should note
  the new element.
- **Stub / impure detectors** — anything that treats "no effects" as pure/stub
  now correctly classifies an `@Outgoing`/`Emitter.send` method as impure. Audit
  callers of `EffectSet.Has` / empty-set checks so a publisher is no longer
  mistaken for a pure function.

---

## 3. Blast radius — touchpoints of a NEW entity kind

Adding `SCOPE.ChannelBinding` touches every place that enumerates or
allow-lists entity kinds. Checklist (do not miss one — each is grounded):

1. **`internal/types/kinds.go`** — declare `EntityKindChannelBinding`
   (near kinds.go:76) and append to `AllEntityKinds()` (kinds.go:339). The
   `AllEntityKinds` test (`kinds_test.go`) will fail until the map/name is
   added — this is the guard that catches a leaked free-form kind.
2. **Noise classification — `internal/mcp/denoise.go`** (`classifyNoise`,
   denoise.go:90). ChannelBinding is a real, lined-to-config structural signal,
   **not** noise — do NOT add it to a noise bucket, but confirm it does not
   accidentally match the `SCOPE.Pattern`/bare-`pattern` rule (denoise.go:108)
   or schema-field rule. Add a one-line "explicitly not noise" test.
3. **Dead-code allow-list — `internal/mcp/dead_code.go`**, map
   `frameworkEntryKindsMCP` (dead_code.go:184-193). `SCOPE.MessageTopic` is
   already an entry-kind (dead_code.go:189). A ChannelBinding is a config-side
   declaration with no callers by design — add `"SCOPE.ChannelBinding": true`
   so it is never reported as dead code.
4. **Dashboard topology — `internal/dashboard/handlers_topology.go`**
   (`kindMessageTopic`, handlers_topology.go:23; switch at handlers_topology.go:120).
   Decide whether ChannelBinding renders as its own topology node or is folded
   as an edge annotation between Operation and MessageTopic. Recommendation:
   fold it as edge metadata (keeps the topology graph channel/topic-centric),
   but expose its orphan flags (§1.4) via the existing orphan scans.
5. **Docgen** — no `MessageTopic` reference exists under `internal/docgen`
   today (grep clean), so ChannelBinding needs no docgen template change for
   v0.1.8.1. Revisit when messaging gets a dedicated doc tier.
6. **Coverage** — §4.
7. **FlatBuffers graph schema** — the kind is a string field, no `.fbs` change
   (`internal/graph/schema/graph.fbs` stores `Kind` as a string), so binary
   format is unaffected.

---

## 4. Coverage matrix update

### 4.1 How it is generated/stored (verified)

- Source of truth: **`docs/coverage/registry.json`** (`$schema_version` + 821
  `records`). Each record: `id`, `category`, `subcategory`, `language`, `label`,
  and `capabilities` = `{ Category: { cap_key: {status, cites, issue, notes,
  verified_at} } }`. `status ∈ {full, partial, missing}`.
- Capability vocabulary: **`tools/coverage/capability-dictionary.yaml`**. The
  `message_broker` category declares
  `capabilities: [producer_extraction, consumer_extraction, topic_attribution,
  room_channel_grouping, signature_verification]` (dictionary line ~57) and
  `subcategory_order: [schedulers, task_queues, brokers, realtime_channels,
  webhooks]`. Each subcategory (e.g. `brokers`, line ~963) re-declares the
  subset of capability columns it carries.
- Generation: `go run ./tools/coverage gen` renders `docs/coverage/*.md`
  (`summary.md`, `by-language/**`, `by-category/**`, `detail/**`) from the JSON.
  CI (`.github/workflows/coverage-docs.yml`) runs `validate`, `backfill --check`,
  `fmt --check`, then `gen` and fails if `docs/coverage/` is not in sync.
- The site component (`site/src/components/CoverageMatrix.astro`) is a static
  marketing shell fed by `site/src/scripts/coverage.js`; it renders aggregate
  chips/bars, not per-capability cells — **no change needed** for this ADR.

### 4.2 The concrete change

Add **one new capability dimension** to the `message_broker` category to
represent config↔code↔topic binding + the publish effect. Two options; pick
**A** (single combined column) for v0.1.8.1 to keep the pivot dense:

**Option A — one capability `config_binding`:**
1. `tools/coverage/capability-dictionary.yaml`: append `config_binding` to
   `message_broker.capabilities` (line ~58) and to the `brokers` subcategory
   `capabilities` list (line ~963) — `msg.broker.kafka` is
   `subcategory: brokers`. Add a dictionary description entry.
2. `docs/coverage/registry.json`: on the reference records
   (`msg.broker.kafka`, and the Quarkus/SmallRye rows) set
   `capabilities.<Category>.config_binding` (or a new `Messaging`/`Binding`
   category — reuse the existing `message_broker` cell category the other
   caps live under) to:
   ```json
   "config_binding": {
     "status": "full",
     "cites": [
       "internal/extractors/config/discover.go",
       "internal/custom/java/microprofile.go",
       "internal/links/topic_pass.go"
     ],
     "issue": "5782",
     "notes": "SmallRye/Quarkus reference impl: SCOPE.ChannelBinding joins mp.messaging.*.topic config to @Incoming/@Outgoing channels and to kafka: MessageTopic.",
     "verified_at": "2026-07-16"
   }
   ```
   Every other framework in `brokers` gets `status: "missing", issue: "5782"`
   until §5 lands it. **SmallRye/Quarkus is the first ✅.**
3. (If a distinct message-publish-effect dimension is wanted) add a second
   capability `message_publish_effect` the same way, cited to
   `internal/substrate/effects.go` + `effect_sinks_java.go` +
   `effect_sinks_kotlin.go`. For v0.1.8.1 keep it folded into `config_binding`
   notes to avoid a near-empty column tripping the #4007 column-strand guard.
4. Run `go run ./tools/coverage validate && go run ./tools/coverage gen`,
   commit the regenerated `docs/coverage/**`. Do **not** hand-edit the `.md`.

Do not invent a new top-level matrix format — this is purely a new capability
key inside the existing `message_broker` category + subcategory allow-lists.

---

## 5. Generalization roadmap

### 5.1 The framework-agnostic seam

Two pluggable recognizer families sharing one entity model:

- **Shared model**: `SCOPE.ChannelBinding` (§1) + the `BINDS` / `BINDS_TOPIC`
  edges + the message_publish effect — all framework-neutral.
- **Config-side recognizer** (in/near `config/discover.go`): a table of
  `(subtype, keyPattern → {channel, direction, connector, topic})` extractors.
  SmallRye is the first entry (`mp.messaging.*`). Each new framework is a new
  key-pattern row gated by config subtype.
- **Code-side recognizer** (custom extractors): annotation/decorator patterns
  that stamp `channel` + `direction` on an Operation, exactly as
  `ExtractMicroProfile` does (microprofile.go:158). The `BINDS` resolver joins
  config→code purely on the shared `channel` property, so any framework that
  populates it participates for free.

The seam is: **config recognizer emits ChannelBinding with a `channel`; code
recognizer stamps `channel` on an Operation; resolver joins them.** No framework
special-casing below the recognizer layer.

### 5.2 Prioritized backlog

| Prio | Framework | Config / annotation binding source | Effort |
|---|---|---|---|
| 0 (ships) | **Quarkus / SmallRye / Kafka** | `mp.messaging.{in,out}.<ch>.{connector,topic}` + `@Incoming`/`@Outgoing` | reference |
| 1 | **Spring Cloud Stream** | `spring.cloud.stream.bindings.<name>.destination` (+ `.binder`) ↔ `@Bean Function/Consumer/Supplier` or `StreamBridge.send` | S (config parser mirrors SmallRye; same `spring_properties` gate) |
| 2 | **Spring Kafka** | `@KafkaListener(topics=…)` / `@KafkaHandler` / `KafkaTemplate.send(topic,…)` — topic is **inline in the annotation**, minimal config join | S (annotation-only; code recognizer carries channel=topic) |
| 3 | **NestJS Kafka / microservices** | `@MessagePattern` / `@EventPattern` + `ClientKafka.emit()`; topics in `createMicroservice({transport:Transport.KAFKA})` module config | M (TS decorators + module-config topic map) |
| 4 | **Node kafkajs** | no annotations — `consumer.subscribe({topic})` / `producer.send({topic})` call-site literals | M (call-arg literal extraction, no config file) |
| 5 | **Go (Sarama / segmentio/kafka-go / confluent-kafka-go / watermill)** | topic is a string arg to `ConsumePartition` / `(*Writer).WriteMessages` / `Subscribe`; watermill router handler topics | M–L (per-client call-site patterns; `msg.broker.kafka-go` record exists) |
| 6 | **Python (aiokafka / faust / celery)** | faust `@app.agent(topic)`; aiokafka `send/subscribe(topic)`; celery `task_routes` config + `@app.task` | M (`msg.kafka-streams` Faust record exists) |
| 7 | **Rails (Sidekiq / ActiveJob / Karafka)** | `sidekiq.yml` queues, `queue_as`, Karafka `topic :name` DSL routes | L (Ruby DSL + YAML config join) |

Effort tiers: **S** = new config key-pattern row + reuse resolver;
**M** = new code-side call-site/decorator recognizer; **L** = new DSL/config
grammar. Frameworks whose topic is inline in code (kafkajs, Go, Spring Kafka)
skip the config-side recognizer and bind channel≡topic directly.

---

## 6. Sequencing + validation plan

### 6.1 Implementation order

1. `SCOPE.ChannelBinding` kind + `AllEntityKinds` + blast-radius allow-lists
   (§3.1–§3.3, §3.7). Cheap, unblocks everything; guarded by `kinds_test`.
2. `discoverChannelBindings` recognizer + entity emission (§1.2). Validate the
   entity appears with correct props before wiring edges.
3. `BINDS` / `BINDS_TOPIC` edges + resolver join (§1.3). Validate the
   Operation↔ChannelBinding↔MessageTopic path resolves.
4. Orphan detection (§1.4) + topology surfacing (§3.4).
5. message_publish effect element + Java/Kotlin sniffers (§2). Independent of
   1–4; can land in parallel.
6. Coverage matrix update + `gen` (§4).
7. Generalization backlog (§5.2) — post-v0.1.8.1.

### 6.2 Live validation on `event-driven-ai` corpus

The `event-driven-ai` corpus is the SmallRye/Quarkus/Kafka reference workload.
Validate each step against it:

- **Steps 1–4 (ChannelBinding)** require a **Phase 2 re-index** — the config
  recognizer runs at index time, so bindings only appear after re-indexing.
  Validation queries: `grafel_find kind_filter=ChannelBinding` returns one per
  `mp.messaging.*` group; `grafel_inspect` on an `@Outgoing` Operation shows the
  `BINDS`→ChannelBinding→`BINDS_TOPIC`→`kafka:<topic>` chain;
  `grafel_orient view=topology` (the #5781 channel listing) now shows topic
  names next to channels.
- **Step 5 (effect)** requires a **second re-index** because effects are
  pre-baked (§2.3). Validate: `grafel_effects` on an `Emitter.send` method lists
  `message_publish`; a previously "pure-looking" `@Outgoing` method is now impure.
- **Step 6** validated by `go run ./tools/coverage validate && gen` + CI
  (`coverage-docs.yml`).

### 6.3 Risk

- **Re-index is mandatory twice** (bindings, then effects). Any group not
  re-indexed shows stale/absent signal — document in release notes. `effectSlots`
  bump changes the sidecar bit-width; **old effect sidecars must be regenerated**,
  not merged (the 8th bit shifts nothing since it is appended, but the
  `effectSlots==len(AllEffects())` guard will reject a stale 7-slot sidecar).
- **Value capture exception** (§1.2) diverges from the value-discard posture of
  `parseProperties`/`parseEnv`. Scope it strictly to connector/topic/serializer
  keys; never widen the regex to arbitrary values (secrets live in the same
  files).
- **Channel-name collisions**: `BINDS` joins on `channel` within a repo; two
  operations claiming the same channel/direction is a real modeling error — the
  orphan/duplicate detector (§1.4) should flag rather than silently pick one.

---

## What ships in v0.1.8.1 vs generalization backlog

**Ships in v0.1.8.1 (reference slice):**
- `SCOPE.ChannelBinding` kind + all blast-radius allow-lists (§3).
- SmallRye/Quarkus config recognizer `discoverChannelBindings`
  (`quarkus_properties` + `spring_properties` gate) with `channel/direction/
  connector/topic/serializer/source_config`.
- `BINDS` (→Operation by channel) + `BINDS_TOPIC` (→`kafka:`+topic MessageTopic)
  edges + producer-less/consumer-less/dangling-topic detection.
- `message_publish` effect element + Java (`Emitter.send`/`@Outgoing`) and
  Kotlin sniffers.
- Coverage: `config_binding` capability on `message_broker`/`brokers`,
  SmallRye/Quarkus = first ✅, all other brokers `missing #5782`.
- Two-phase re-index of `event-driven-ai` as the live validation.

**Generalization backlog (post-v0.1.8.1):**
- Framework-agnostic seam is in place at ship, but only SmallRye is wired.
- Next recognizers in priority order: Spring Cloud Stream (S) → Spring Kafka (S)
  → NestJS (M) → kafkajs (M) → Go clients/watermill (M–L) → Python
  aiokafka/faust/celery (M) → Rails Sidekiq/ActiveJob/Karafka (L).
- Optional split of `message_publish_effect` into its own coverage column once
  ≥2 frameworks carry it (avoids the column-strand guard tripping on a lone cell).
