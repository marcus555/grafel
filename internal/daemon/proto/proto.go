// Package proto defines the wire types exchanged between the archigraph
// CLI client and the long-running daemon over the Unix-domain socket.
//
// Per ADR-0017 the transport is net/rpc with the jsonrpc codec. Service
// methods follow the net/rpc convention: exactly one argument struct and
// one reply pointer; service name is "Daemon".
//
// All types in this package MUST stay backwards-compatible — new fields
// may be added, existing fields must not change meaning. The daemon and
// the client binary may be at different versions during an upgrade.
package proto

// ServiceName is the net/rpc service name registered by the daemon.
const ServiceName = "Daemon"

// PingArgs is the no-op liveness probe argument.
type PingArgs struct{}

// PingReply carries the daemon's self-reported version.
type PingReply struct {
	Version string `json:"version"`
}

// StatusArgs requests a snapshot of daemon state.
type StatusArgs struct{}

// StatusReply summarises the daemon's runtime state. Fields are added
// here as new phases land; clients must tolerate missing fields.
type StatusReply struct {
	Version    string   `json:"version"`
	PID        int      `json:"pid"`
	UptimeSec  int64    `json:"uptime_sec"`
	RSSBytes   uint64   `json:"rss_bytes"`
	InFlight   int      `json:"in_flight"`
	Groups     []string `json:"groups,omitempty"`
	StartedAt  string   `json:"started_at"`
	SocketPath string   `json:"socket_path"`
}

// IndexArgs requests a one-shot index of a single repository. Mirrors the
// flags of the old `archigraph index <repo>` subcommand.
type IndexArgs struct {
	RepoPath    string   `json:"repo_path"`
	OutPath     string   `json:"out_path,omitempty"`
	RepoTag     string   `json:"repo_tag,omitempty"`
	SkipPasses  []string `json:"skip_passes,omitempty"`
	Pretty      bool     `json:"pretty,omitempty"`
	JSONStats   bool     `json:"json_stats,omitempty"`
	Repair      bool     `json:"repair,omitempty"`
	RepairApply bool     `json:"repair_apply,omitempty"`
	ExportFB    bool     `json:"export_fb,omitempty"`
}

// IndexReply carries the post-index summary. The stats are an opaque
// JSON blob (the same shape the old `--json-stats` flag printed) so the
// daemon can extend them without forcing a client release.
type IndexReply struct {
	RepoPath  string `json:"repo_path"`
	GraphPath string `json:"graph_path,omitempty"`
	StatsJSON string `json:"stats_json,omitempty"`
}

// RebuildArgs requests a force-rebuild of a group (optionally narrowed
// to a single repo slug).
type RebuildArgs struct {
	Group string `json:"group"`
	Slug  string `json:"slug,omitempty"`
	Wipe  bool   `json:"wipe,omitempty"`
}

// RebuildReply lists the repos that were rebuilt and any warning that
// applies to the whole batch.
type RebuildReply struct {
	Repos   []string `json:"repos"`
	Warning string   `json:"warning,omitempty"`
}

// StopArgs requests a graceful shutdown of the daemon.
type StopArgs struct{}

// StopReply is empty; the daemon closes the socket once the reply is
// flushed and exits when the last in-flight request finishes.
type StopReply struct{}
