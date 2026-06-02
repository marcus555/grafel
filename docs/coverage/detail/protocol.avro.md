<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `protocol.avro` — Apache Avro (.avsc / .avpr)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [protocol](../by-category/protocol.md)
- **Capability cells:** 5

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Cross repo linkage | 🔴 `missing` | — | 3690 | — | — |
| Field extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/avro/avro.go`<br>`internal/extractors/avro/avro_test.go` | #3690 — each field -> SCOPE.Schema/field (Name=<Record>.<field>, Properties.type incl. array<T>/map<T>/union a|b) + record->named-type REFERENCES edge (array items followed); primitives carry no edge |
| Method attribution | 🔴 `missing` | — | 3690 | — | — |
| Schema extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/avro/avro.go`<br>`internal/extractors/avro/avro_test.go` | #3690 — record/enum/fixed -> SCOPE.Schema; .avpr protocol envelope 'types' walked; namespaces stripped to local name |
| Service extraction | 🔴 `missing` | — | 3690 | — | Avro protocol (.avpr) messages not yet emitted as services; #3690 covers data-contract types only |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update protocol.avro ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
