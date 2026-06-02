<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `protocol.grpc` — gRPC

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [protocol](../by-category/protocol.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Cross repo linkage | ✅ `full` | `2026-06-02` | 3686 | `internal/engine/grpc_edges.go`<br>`internal/engine/grpc_edges_test.go`<br>`internal/links/grpc_pass.go` | Client stub calls emit SCOPE.GrpcMethod entities keyed grpc:<Service>/<Method> + GRPC_HANDLES edges matching the server identity; P6 (links/grpc_pass.go) joins client↔server across repos. Node/TS client coverage now includes the modern factory-function stubs nice-grpc (createClient(<Service>Definition, ch)) and Connect/connectrpc (createPromiseClient(<Service>Service, transport)) alongside classic @grpc/grpc-js constructor stubs. |
| Method attribution | ✅ `full` | `2026-06-02` | — | `internal/engine/grpc_edges.go`<br>`internal/engine/grpc_edges_test.go` | — |
| Service extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/grpc_edges.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update protocol.grpc ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
