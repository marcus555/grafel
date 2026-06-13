<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.wiremock` — wiremock

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 5008 | `internal/engine/tests_edges.go`<br>`internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/resolver.go` | #5008: wiremock async HTTP mock server (MockServer::start + Mock::given(matcher).respond_with(ResponseTemplate::new).mount) detection added to rust/test_patterns.yaml; dev-dependency manifest parsing, same model as test.mockall. |
| Target extraction | ✅ `full` | `2026-06-14` | 5008 | `internal/engine/rules/rust/test_patterns.yaml`<br>`internal/extractors/cross/testmap/frameworks.go` | #5008: wiremock async HTTP mock server (MockServer::start + Mock::given(matcher).respond_with(ResponseTemplate::new).mount) detection added to rust/test_patterns.yaml; dev-dependency manifest parsing, same model as test.mockall. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.wiremock ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
