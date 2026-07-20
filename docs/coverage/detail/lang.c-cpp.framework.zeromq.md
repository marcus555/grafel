<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.framework.zeromq` — ZeroMQ (libzmq/cppzmq)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Subcategory:** Brokers
- **Capability cells:** 4

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Config binding | 🔴 `missing` | — | 5782 | — | — |
| Consumer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | Literal endpoints only; SUB/PULL socket roles + connect→subscriber heuristic. |
| Producer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | Literal endpoints only; PUB/PUSH socket roles + bind→publisher heuristic. No const/config resolution. |
| Topic attribution | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | Endpoint-keyed MessageTopic (zmq:<endpoint>); ZeroMQ has no broker-side topic, endpoints joined cross-repo. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.framework.zeromq ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
