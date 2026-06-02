<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.rust.orm.sqlx` — sqlx (Rust)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 10

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/rust/orms/sqlx.yaml` | — |
| Schema extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/rust/sqlx_rbatis.go`<br>`internal/custom/rust/sqlx_rbatis_test.go`<br>`internal/custom/rust/testdata/sqlx_models.rs` | Detects #[derive(FromRow)] structs and Pool::connect() as schema-mapped models |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Foreign key extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Lazy loading recognition | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Relationship extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-05-30` | — | `internal/custom/rust/orm_props_test.go`<br>`internal/custom/rust/sqlx_rbatis.go` | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🟢 `partial` | `2026-05-30` | — | `internal/custom/rust/sqlx_rbatis.go`<br>`internal/custom/rust/sqlx_rbatis_test.go` | sqlx::migrate! path + migration file refs detected; parsing the referenced migrations/*.sql DDL is out of scope (sqlx resolves them at compile time, not in the .rs source). |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.rust.orm.sqlx ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
