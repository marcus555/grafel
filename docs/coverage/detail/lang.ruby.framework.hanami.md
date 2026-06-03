<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.ruby.framework.hanami` — Hanami

Auto-generated. Back to [summary](../summary.md).

- **Language:** [ruby](../by-language/ruby.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Backend HTTP
- **Capability cells:** 49

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🔴 `missing` | — | 3628 | — | — |
| Endpoint pagination posture | 🔴 `missing` | `2026-06-02` | 3628 | `internal/engine/http_endpoint_pagination.go`<br>`internal/engine/http_endpoint_pagination_patterns.go`<br>`internal/engine/http_endpoint_pagination_test.go`<br>`internal/engine/http_endpoint_synthesis.go` | #3628: applyEndpointPagination stamps paginated/pagination_style/pagination_params via the cross-language parameters/parameter_schema fallback (limit+offset/page/cursor shape). No framework-specific pagination-class/ORM signal yet for this framework. |
| Endpoint response codes | 🔴 `missing` | — | 3818 | — | — |
| Endpoint synthesis | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/ruby/frameworks/hanami.yaml` | — |
| Handler attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/ruby/frameworks/hanami.yaml` | — |
| Route extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/routes.go` | — |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | view_rendering:#3628-not-yet-extracted | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | 🟢 `partial` | `2026-05-30` | — | `internal/custom/ruby/auth.go` | — |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/validation.go` | — |
| Request validation | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/validation.go` | — |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | 🟢 `partial` | — | — | `internal/custom/ruby/middleware.go` | Rack use via Hanami::Application, config.middleware.use. Part of #3282. |
| Rate limit stamping | 🟢 `partial` | `2026-06-03` | — | `internal/custom/ruby/rate_limit_endpoint.go`<br>`internal/custom/ruby/rate_limit_endpoint_test.go` | rack-attack 'Rack::Attack.throttle(name, limit: N, period: T)' detected and stamped as a SCOPE.Pattern/rate_limit marker (rate_limited/limit/period/rate_limit_name/rate_limit_source=rack-attack; literal period 1.minute->60 -> rate_limit '<N>/<secs>s'). Rack middleware applies to this Rack-based framework. PARTIAL: rack-attack throttles bind to a request discriminator (the block), not a named route, so the per-route binding is heuristic (rate_limit_scope=request); blocklist/safelist are not stamped as limits. Framework-native limiter idioms are future work. #4072 |

### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type graph extraction | — `not_applicable` | — | — | — | GraphQL schema type→type graph (object-typed field -> referenced object type with list/nullable cardinality) is a GraphQL-only concept; this framework is not a GraphQL server, so it has no GraphQL object-type relationship graph. |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | — `not_applicable` | — | — | — | Ruby is dynamically typed — no enum keyword (duck typing idiom) |
| Interface extraction | — `not_applicable` | — | — | — | Ruby is dynamically typed — no interface keyword (duck typing idiom) |
| Type alias extraction | — `not_applicable` | — | — | — | Ruby is dynamically typed — no type keyword (duck typing idiom) |
| Type extraction | — `not_applicable` | — | — | — | Ruby dynamically typed; framework exposes no static type DSL |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | 3628 | — | — |
| DI injection point | 🔴 `missing` | — | 3628 | — | — |
| DI scope resolution | 🔴 `missing` | — | 3628 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/extractors/cross/testmap/frameworks.go` | — |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/observability.go` | — |
| Metric extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/observability.go` | — |
| Trace extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/ruby/observability.go` | — |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-05-28` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractor/config_key.go`<br>`internal/extractors/ruby/config_consumer.go`<br>`internal/extractors/ruby/config_consumer_test.go` | ENV[...], ENV.fetch -> config:<key> DEPENDS_ON_CONFIG edges (issue #3641) |
| Constant propagation | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/ruby.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_ruby.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_ruby.go` | — |
| Def use chain extraction | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use_ruby.go` | — |
| Env fallback recognition | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/ruby.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-03` | — | `internal/extractor/exception_flow.go`<br>`internal/extractors/ruby/exception_flow.go`<br>`internal/extractors/ruby/exception_flow_test.go` | raise X / raise Mod::X -> THROWS; rescue A, B => e / method-level rescue / Rails rescue_from A, B, with: -> CATCHES; bare rescue catch-all + string raise + bare re-raise dropped (#3628) |
| Feature flag gating | ✅ `full` | `2026-06-03` | 4140 | `internal/engine/feature_flag_edges.go`<br>`internal/engine/feature_flag_edges_test.go`<br>`internal/engine/orm_queries.go` | Ruby flag-check call sites -> feature:<key> + GATED_BY (Flipper symbol/subscript/feature() + Unleash is_enabled? + Rollout active? + LaunchDarkly variation); framework-agnostic engine pass, value-asserted Ruby unit tests |
| Fs effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_ruby.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_ruby.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/ruby.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/module_cycle_pass.go` | — |
| Mutation effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_ruby.go` | — |
| Pure function tagging | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/pure_function_pass.go` | — |
| Reachability analysis | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_ruby.go` | — |
| Request shape extraction | 🟢 `partial` | `2026-06-03` | 3951 | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_ruby.go`<br>`internal/substrate/payload_shapes_t2_test.go` | GENUINE-PARTIAL (#3951): Hanami action request = bare `params[:x]` reads inside `def call(params)`, bound to the `call` header (Hanami uses `def call`, so the stock def-header path already fires). Test TestPayloadShapesRuby_HanamiAction asserts {email,name}. PARTIAL: dry-validation schema blocks not parsed. DEPLOY-DEFERRED. |
| Request sink dataflow | 🔴 `missing` | — | 3740 | — | — |
| Response shape extraction | 🟢 `partial` | `2026-06-03` | 3951 | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_ruby.go`<br>`internal/substrate/payload_shapes_t2_test.go` | GENUINE-PARTIAL (#3951): Hanami action response = inline `JSON.generate({...})` / `JSON.dump({...})` body (commonly `self.body = JSON.generate({...})`), bound to the `def call` header. Test TestPayloadShapesRuby_HanamiAction asserts {id,email}. PARTIAL: view-rendered bodies (no inline hash) and to_json on a var remain unresolved. DEPLOY-DEFERRED. |
| Sanitizer recognition | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_ruby.go` | — |
| Schema drift detection | 🟢 `partial` | `2026-06-03` | 3951 | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_ruby.go` | Framework-agnostic drift pass now has genuine Hanami request + response shapes to compare (#3951). |
| Taint sink detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_ruby.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_ruby.go` | — |
| Template pattern catalog | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern_ruby.go` | — |
| Vulnerability finding | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_ruby.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.ruby.framework.hanami ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
