<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `platform.rust.shuttle` — Shuttle (deploy runtime)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** IaC / Provisioning
- **Capability cells:** 9

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac cross stack reference | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac environment region account | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac event source wiring | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac iam grant attribution | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac output export extraction | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Iac resource property extraction | 🟢 `partial` | `2026-06-14` | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: resource_provider/resource_type props stamped on each Shuttle managed-resource component; deeper per-resource config (db size/region) not modelled. |
| Iac stack app topology | — `not_applicable` | — | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: not applicable to Shuttle's attribute-macro runtime-provisioning model (no IaC stack graph / cross-stack refs / IAM grants); only entrypoint + managed-resource declarations are surfaced (resource_extraction). |
| Resource extraction | 🟢 `partial` | `2026-06-14` | 5008 | `internal/custom/rust/ntex_loco_shuttle.go`<br>`internal/custom/rust/ntex_loco_shuttle_test.go` | #5008: Shuttle managed-resource extraction — #[shuttle_runtime::main] entrypoint -> SCOPE.Service (deploy_runtime=shuttle, entrypoint=fn) and each #[shuttle_*::Resource] annotation (shuttle_shared_db::Postgres, shuttle_secrets::Secrets, shuttle_aws_rds::*, shuttle_persist::*, …) -> SCOPE.Component (resource_provider/resource_type). Gated on a `shuttle_` token; the runtime::main macro is excluded from the resource set. Value-asserted: TestShuttleRuntimeAndResources (+ wrong-language + no-match no-ops). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update platform.rust.shuttle ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
