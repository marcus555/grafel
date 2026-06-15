# Daemon Operational Modes (S7 of #2149)

The grafel daemon supports three operational modes that control memory usage,
background activity, and feature activation. The mode is persisted in
`~/.grafel/daemon.config.json` and read on every boot.

## Modes

### background (default)

Low-footprint preset designed for open-source / first-time installs and
resource-constrained machines.

| Env var | Default |
|---------|---------|
| `GRAFEL_EAGER_ALGO` | `false` — algo passes run on-demand only |
| `GRAFEL_EMBEDDING_URL` | `` (empty) — MiniLM embeddings disabled |
| `GRAFEL_HEAP_MAX_PCT` | `60` — heap capped at 60% of available memory |

### workstation

Restores the historical production defaults: eager algo passes, no heap cap
override, embedding endpoint is freely configurable.

| Env var | Default |
|---------|---------|
| `GRAFEL_EAGER_ALGO` | `true` |
| `GRAFEL_HEAP_MAX_PCT` | `80` |

### readonly

Serves graph queries against the existing `graph.fb` only. No reindex, no
watcher subscriptions, no algo passes. Use this when you want fast read-only
access without any background CPU or memory pressure.

| Env var | Default |
|---------|---------|
| `GRAFEL_DISABLE_WATCHER` | `true` |
| `GRAFEL_DISABLE_REBUILD` | `true` |
| `GRAFEL_DISABLE_ALGO` | `true` |

## Precedence

Env vars set in the process environment **always** take precedence over the mode
defaults. This lets operators fine-tune a single variable without switching modes:

```
GRAFEL_EAGER_ALGO=true grafel daemon --mode=background
```

In the example above the daemon runs in background mode except that eager algo is
enabled.

## CLI reference

```
# Pick mode at install time (default: background)
grafel install --mode=workstation

# Override mode at daemon start
grafel daemon --mode=readonly

# Switch mode persistently (saves config + restarts daemon)
grafel mode background
grafel mode workstation
grafel mode readonly

# Show current mode
grafel status
```

`grafel status` reports the active mode in the daemon header line:

```
Daemon: running  pid=12345  uptime=2h3m  rss=180.4MB  in_flight=0
  version: 1.x.y
  socket:  /Users/you/.grafel/sockets/daemon.sock
  mode:    background
  dashboard: http://127.0.0.1:47274/
```

## Config file

`~/.grafel/daemon.config.json` persists the active mode plus any
operator-supplied env overrides written by `grafel mode`:

```json
{
  "mode": "background",
  "env_overrides": {
    "GRAFEL_HEAP_MAX_PCT": "50"
  }
}
```

The file is written atomically (write to `.tmp` then rename) and is
backwards-compatible: daemons predating S7 simply ignore it.
