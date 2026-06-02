<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.sidekiq` — Sidekiq (Ruby task queue)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [ruby](../by-language/ruby.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🟢 `partial` | — | 3700 | `internal/engine/scheduled_jobs_edges.go`<br>`internal/engine/scheduled_jobs_edges_test.go` | #3700 (epic #3628 area #14): a class including Sidekiq::Worker/Job with def perform becomes a SCOPE.ScheduledJob (sidekiq:<Worker>) with a TRIGGERS edge to perform (the job consumer). sidekiq-cron Sidekiq::Cron::Job.create(cron:,class:) attaches the cron schedule to the same worker job. Honest-partial: requires include + def perform in one file; YAML-loaded cron schedules and dynamic class refs not yet parsed. |
| Producer extraction | 🟢 `partial` | — | 3700 | `internal/engine/scheduled_jobs_edges.go`<br>`internal/engine/scheduled_jobs_edges_test.go` | #3700 (epic #3628 area #14): Worker.perform_async/perform_in/perform_at/perform_bulk dispatch sites emit an ENQUEUES edge (new kind) from the enclosing Ruby method to the worker job entity (SCOPE.ScheduledJob sidekiq:<Worker>). Honest-partial: enqueue target resolves only when the worker class def is in the indexed group; dynamic class names not resolved. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.sidekiq ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
