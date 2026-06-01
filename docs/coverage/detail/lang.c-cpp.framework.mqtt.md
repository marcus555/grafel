<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.framework.mqtt` — MQTT (Paho C/C++ / Mosquitto)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | Literal topics via mosquitto_subscribe / MQTTClient_subscribe / Paho C++ client.subscribe. |
| Producer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | Literal topics via mosquitto_publish / MQTTClient publish / Paho C++ client.publish. |
| Topic attribution | 🟢 `partial` | `2026-05-31` | — | `internal/engine/cpp_messaging_edges.go` | mqtt:<topic> node (supports +/# wildcards); cross-repo joinable. Literal-only. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.framework.mqtt ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
