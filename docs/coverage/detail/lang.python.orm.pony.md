<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.orm.pony` — Pony ORM

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 10

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/python/orms/pony_orm.yaml` | — |
| Schema extraction | 🟢 `partial` | `2026-05-29` | 3072 | `internal/custom/python/orm_schema.go` | Pony ORM entity field definitions (Required, Optional etc.) are not extracted; only model class detection and query attribution are handled. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🟢 `partial` | — | 3070 | `internal/custom/python/orm_relationships.go`<br>`internal/custom/python/testdata/pony_relationships.py` | — |
| Foreign key extraction | 🟢 `partial` | — | 3070 | `internal/custom/python/orm_relationships.go`<br>`internal/custom/python/testdata/pony_relationships.py` | — |
| Lazy loading recognition | — `not_applicable` | `2026-05-29` | — | — | Pony ORM uses explicit query interface; no transparent lazy loading (#3184) |
| Relationship extraction | 🟢 `partial` | — | 3070 | `internal/custom/python/orm_relationships.go`<br>`internal/custom/python/testdata/pony_relationships.py` | — |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/python/orms/pony_orm.yaml` | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | Pony ORM has no built-in migration support; schema changes require manual intervention (#3184) |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.orm.pony ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
