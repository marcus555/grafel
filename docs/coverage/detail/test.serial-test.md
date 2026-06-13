<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.serial-test` — serial_test

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 5008 | `internal/engine/tests_edges.go`<br>`internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/resolver.go` | #5008: serial_test (#[serial]/#[parallel]/#[file_serial]) detection added to rust/test_patterns.yaml; the underlying #[test]/#[tokio::test] fn is already linked to its subject by the shared rustTestRE in frameworks.go (extra-attribute lines tolerated), so a #[serial]-decorated test is attributed without a serial-specific code path. |
| Target extraction | ✅ `full` | `2026-06-14` | 5008 | `internal/engine/rules/rust/test_patterns.yaml`<br>`internal/extractors/cross/testmap/frameworks.go` | #5008: serial_test (#[serial]/#[parallel]/#[file_serial]) detection added to rust/test_patterns.yaml; the underlying #[test]/#[tokio::test] fn is already linked to its subject by the shared rustTestRE in frameworks.go (extra-attribute lines tolerated), so a #[serial]-decorated test is attributed without a serial-specific code path. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.serial-test ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
