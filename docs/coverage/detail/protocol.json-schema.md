<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `protocol.json-schema` — JSON Schema (*.schema.json)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [protocol](../by-category/protocol.md)
- **Capability cells:** 5

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Cross repo linkage | 🔴 `missing` | — | 3690 | — | — |
| Field extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/jsonschema/jsonschema.go`<br>`internal/extractors/jsonschema/jsonschema_test.go` | #3690 — each property -> SCOPE.Schema/field (Name=<Schema>.<prop>, Properties.type incl. array<T>) + $ref -> REFERENCES edge (direct, array items, allOf/anyOf/oneOf branches) |
| Method attribution | 🔴 `missing` | — | 3690 | — | — |
| Schema extraction | ✅ `full` | `2026-06-02` | — | `internal/extractors/jsonschema/jsonschema.go`<br>`internal/extractors/jsonschema/jsonschema_test.go` | #3690 — root object schema (named via title, basename fallback) + each $defs/definitions subschema -> SCOPE.Schema/object; content-sniffed for $schema/properties/$defs/$ref so misrouted JSON is a no-op |
| Service extraction | 🔴 `missing` | — | 3690 | — | JSON Schema is a data-contract format with no service/RPC concept |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update protocol.json-schema ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
