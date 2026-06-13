<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.clojure.framework.ring` — Ring

Auto-generated. Back to [summary](../summary.md).

- **Language:** [clojure](../by-language/clojure.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Backend HTTP
- **Capability cells:** 50

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🔴 `missing` | — | 4910 | — | — |
| Endpoint pagination posture | 🔴 `missing` | — | 4910 | — | — |
| Endpoint response codes | 🔴 `missing` | — | 4910 | — | — |
| Endpoint synthesis | 🔴 `missing` | `2026-06-12` | 4910 | — | Ring IS detected (internal/engine/rules/clojure/frameworks/ring.yaml) and is THE foundational Clojure HTTP abstraction (request-map → response-map handler fn + wrap-* middleware) that Compojure/Reitit/Pedestal build on. Bare Ring handlers carry no static route table (dispatch is in code), so there is no literal route to synthesise — endpoints surface through the Compojure/Reitit routers layered on top (those records carry the route_extraction). Honest: Ring itself is a detected substrate, not a route source. |
| Handler attribution | 🔴 `missing` | — | 4910 | — | — |
| Route extraction | — `not_applicable` | — | — | — | Bare Ring has no declarative route table — routing is imperative handler dispatch. Routes are synthesised from the Compojure/Reitit/Pedestal routers layered on Ring, not from Ring itself. |
| Websocket route extraction | 🔴 `missing` | `2026-06-14` | — | — | #4965: dedicated websocket_route_extraction not yet implemented for this framework. The capability key was introduced for the rust axum/actix/warp WS extractor (internal/custom/rust/websocket_routes.go); this framework's WebSocket-upgrade idiom is not yet recognised and is a follow-up gap. |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | 4910 | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | 🔴 `missing` | — | 4910 | — | — |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | 🔴 `missing` | — | 4910 | — | — |
| Request validation | 🔴 `missing` | — | 4910 | — | — |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | 🔴 `missing` | — | 4910 | — | Ring wrap-* middleware (wrap-params/wrap-json/wrap-defaults/custom (fn [handler] (fn [req] ...))) and the threading-macro middleware chain are NOT yet extracted — the foundational Ring middleware model is the highest-value Middleware follow-up (#4910 tail; mirrors the Reitit :middleware / rate_limit_stamping gap noted on compojure/reitit). |
| Rate limit stamping | 🔴 `missing` | — | 4910 | — | — |

### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type graph extraction | 🔴 `missing` | — | 4910 | — | — |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🔴 `missing` | — | 4910 | — | — |
| Interface extraction | 🔴 `missing` | — | 4910 | — | — |
| Type alias extraction | 🔴 `missing` | — | 4910 | — | — |
| Type extraction | 🔴 `missing` | — | 4910 | — | — |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | 4910 | — | — |
| DI injection point | 🔴 `missing` | — | 4910 | — | — |
| DI scope resolution | 🔴 `missing` | — | 4910 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🔴 `missing` | — | 4910 | — | — |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🔴 `missing` | — | 4910 | — | — |
| Metric extraction | 🔴 `missing` | — | 4910 | — | — |
| Trace extraction | 🔴 `missing` | — | 4910 | — | — |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DB effect | 🔴 `missing` | — | 4910 | — | — |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | 🔴 `missing` | — | 4910 | — | — |
| Config consumption | 🔴 `missing` | — | 4910 | — | — |
| Constant propagation | 🔴 `missing` | — | 4910 | — | — |
| Dead code detection | 🔴 `missing` | — | 4910 | — | — |
| Def use chain extraction | 🔴 `missing` | — | 4910 | — | — |
| Env fallback recognition | 🔴 `missing` | — | 4910 | — | — |
| Error flow | 🔴 `missing` | — | 4910 | — | — |
| Feature flag gating | 🔴 `missing` | — | 4910 | — | — |
| Fs effect | 🔴 `missing` | — | 4910 | — | — |
| HTTP effect | 🔴 `missing` | — | 4910 | — | — |
| Import resolution quality | 🔴 `missing` | — | 4910 | — | — |
| Module cycle detection | 🔴 `missing` | — | 4910 | — | — |
| Mutation effect | 🔴 `missing` | — | 4910 | — | — |
| Pure function tagging | 🔴 `missing` | — | 4910 | — | — |
| Reachability analysis | 🔴 `missing` | — | 4910 | — | — |
| Request shape extraction | 🔴 `missing` | — | 4910 | — | — |
| Request sink dataflow | 🔴 `missing` | — | 4910 | — | — |
| Response shape extraction | 🔴 `missing` | — | 4910 | — | — |
| Sanitizer recognition | 🔴 `missing` | — | 4910 | — | — |
| Schema drift detection | 🔴 `missing` | — | 4910 | — | — |
| Taint sink detection | 🔴 `missing` | — | 4910 | — | — |
| Taint source detection | 🔴 `missing` | — | 4910 | — | — |
| Template pattern catalog | 🔴 `missing` | — | 4910 | — | — |
| Vulnerability finding | 🔴 `missing` | — | 4910 | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.clojure.framework.ring ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
