<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.java.orm.spring-data-jpa` — Spring Data JPA

Auto-generated. Back to [summary](../summary.md).

- **Language:** [java](../by-language/java.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/orm_field_edges.go`<br>`internal/engine/rules/java/orms/spring_data_jpa.yaml` | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/3097) | `internal/custom/java/jpa_fk_lazy.go` | @Column(name/nullable/length) depth parsed via spring_ecosystem.go FK helper; table_name and entity extraction remain via hibernate.go. Full DDL not parsed. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | ✅ `full` | `2026-06-04` | — | `internal/custom/java/hibernate.go`<br>`internal/custom/java/patterns_dispatch.go`<br>`internal/extractors/custom_java_patterns_smoke_test.go` | Spring Data JPA entities are ordinary jakarta.persistence @Entity classes, so hibernate.go handles them identically to the full jpa/hibernate siblings: hibernateFrameworks admits the "spring_data_jpa" token and frameworkMarkers maps org.springframework.data.jpa -> spring_data_jpa, so a Spring Data JPA file dispatches into ExtractHibernate. @OneToMany/@ManyToOne/@OneToOne/@ManyToMany associations emit DEPENDS_ON edges with association_kind through RunCustomExtractors; value-asserting live smoke test TestJavaPatternsSpringDataJpaAssociationLive asserts the Order->LineItem @OneToMany DEPENDS_ON edge with association_kind=OneToMany emits live. |
| Foreign key extraction | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/3097) | `internal/custom/java/jpa_fk_lazy.go` | @JoinColumn(name=) and @ForeignKey(name=) parsed via ExtractJPAFKAndLazy from spring_ecosystem.go; emits SCOPE.Component/foreign_key entities |
| Lazy loading recognition | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/grafel/issues/3097) | `internal/custom/java/jpa_fk_lazy.go` | FetchType.LAZY and FetchType.EAGER parsed from association annotations; emits SCOPE.Component/fetch_config entities |
| Relationship extraction | ✅ `full` | `2026-06-04` | — | `internal/custom/java/hibernate.go`<br>`internal/custom/java/patterns_dispatch.go`<br>`internal/extractors/custom_java_patterns_smoke_test.go` | Same JPA association code path as the full jpa/hibernate siblings: Spring Data JPA @Entity associations emit directed DEPENDS_ON relationship edges between entities through RunCustomExtractors (spring_data_jpa is in hibernateFrameworks and frameworkMarkers). Value-asserting live smoke test TestJavaPatternsSpringDataJpaAssociationLive asserts the Order->LineItem DEPENDS_ON edge emits live. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/orm_queries.go` | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | `2026-05-29` | — | — | ORM model-definition layer; database migration files are owned by Flyway/Liquibase, not the ORM itself. Same rationale as lang.java.orm.jooq and lang.java.orm.neo4j N/A. |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.java.orm.spring-data-jpa ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
