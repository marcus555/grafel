<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.broker.pulsar` — Apache Pulsar

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Subcategory:** Brokers
- **Capability cells:** 4

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Config binding | 🔴 `missing` | — | 5782 | — | — |
| Consumer extraction | 🟢 `partial` | `2026-05-28` | — | `internal/engine/pulsar_edges.go` | — |
| Producer extraction | 🟢 `partial` | `2026-05-28` | — | `internal/engine/pulsar_edges.go` | — |
| Topic attribution | 🟢 `partial` | `2026-06-02` | — | `internal/engine/pulsar_edges.go`<br>`internal/links/topic_pass.go` | topic:pulsar:<uri> SCOPE.MessageTopic node; topic_pass.go joins a PUBLISHES_TO producer to a SUBSCRIBES_TO consumer sharing the node Name into a cross-repo producer->consumer topology edge (channel=pulsar). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.broker.pulsar ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
