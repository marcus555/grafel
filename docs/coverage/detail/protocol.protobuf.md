<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `protocol.protobuf` — Protocol Buffers

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [protocol](../by-category/protocol.md)
- **Capability cells:** 5

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Cross repo linkage | 🟢 `partial` | `2026-05-28` | — | `internal/engine/grpc_edges.go` | — |
| Field extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/proto/fields_test.go`<br>`internal/extractors/proto/proto.go` | #3690 — each message field -> SCOPE.Schema/field entity (Name=<Msg>.<field>, Properties.type/label) + message->type REFERENCES edge for named (non-scalar) types incl. map<K,V> value; scalars carry no edge |
| Method attribution | ✅ `full` | `2026-05-28` | — | `internal/extractors/proto` | — |
| Schema extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/proto/proto.go`<br>`internal/extractors/proto/proto_test.go` | #3690 — message/enum -> SCOPE.Schema with file CONTAINS edges; messages_and_enums covered by buildMessage/buildEnum |
| Service extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/protobuf/_manifest.yaml`<br>`internal/extractors/proto` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update protocol.protobuf ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
