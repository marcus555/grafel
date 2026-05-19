package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/cajasmota/archigraph/internal/daemon"
	"github.com/cajasmota/archigraph/internal/daemon/proto"
	"github.com/cajasmota/archigraph/internal/registry"
)

// runDaemon is the long-running mode of the archigraph binary. It is
// wired into the CLI as a hidden `archigraph daemon` subcommand —
// users normally reach it via `archigraph start`, which forks this
// process and detaches.
//
// All extractor + registry + linker work happens here. The CLI's other
// subcommands are thin RPC clients (see internal/daemon/client).
func runDaemon(argv []string) error {
	layout, err := daemon.DefaultLayout()
	if err != nil {
		return fmt.Errorf("resolve daemon layout: %w", err)
	}
	if err := daemon.EnsureLayout(layout); err != nil {
		return fmt.Errorf("ensure layout: %w", err)
	}

	// Log to both stderr (so `archigraph start` foreground mode shows
	// progress) and the rotating log file. Phase B will replace the
	// raw file with a size-rotated writer; for Phase A a single append
	// file is fine.
	logFile, err := os.OpenFile(layout.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log %s: %w", layout.LogPath, err)
	}
	defer logFile.Close()
	logger := log.New(io.MultiWriter(os.Stderr, logFile), "archigraph-daemon: ",
		log.LstdFlags|log.Lmicroseconds)

	cfg := daemon.Config{
		Layout:  layout,
		Logger:  logger,
		Index:   daemonIndexFunc,
		Rebuild: daemonRebuildFunc,
	}

	ctx := context.Background()
	return daemon.Run(ctx, cfg)
}

// daemonIndexFunc is the IndexFunc handed to daemon.Run. It bridges the
// RPC argument struct onto the existing in-process Index() entrypoint
// defined in this same package.
func daemonIndexFunc(args proto.IndexArgs) (string, string, error) {
	opts := []IndexOption{
		WithRepairCandidates(args.Repair),
		WithRepairApply(args.RepairApply),
		WithExportFB(args.ExportFB),
	}
	// Capture stats into a local buffer when the caller asked for them.
	// setCapturedStats is a tiny package-level swap (Phase A serializes
	// indexes, so the single-writer assumption holds — see comment in
	// index.go). Phase B's job queue will thread the writer explicitly.
	var statsBuf bytes.Buffer
	if args.JSONStats {
		restore := setCapturedStats(&statsBuf)
		defer restore()
	}
	err := Index(args.RepoPath, args.OutPath, args.RepoTag, args.SkipPasses,
		args.Pretty, args.JSONStats, opts...)
	if err != nil {
		return "", "", err
	}
	graphPath := args.OutPath
	if graphPath == "" {
		graphPath = filepath.Join(args.RepoPath, ".archigraph", "graph.json")
	}
	return graphPath, statsBuf.String(), nil
}

// daemonRebuildFunc force-indexes every repo in a group. We deliberately
// re-implement the iteration here rather than calling into internal/cli
// to avoid pulling cobra back into the daemon's call graph.
func daemonRebuildFunc(args proto.RebuildArgs) ([]string, string, error) {
	groups, err := registry.Groups()
	if err != nil {
		return nil, "", err
	}
	var ref *registry.GroupRef
	for i := range groups {
		if groups[i].Name == args.Group {
			ref = &groups[i]
			break
		}
	}
	if ref == nil {
		return nil, "", fmt.Errorf("unknown group: %s", args.Group)
	}
	cfg, err := registry.LoadGroupConfig(ref.ConfigPath)
	if err != nil {
		return nil, "", err
	}
	var rebuilt []string
	for _, r := range cfg.Repos {
		if args.Slug != "" && r.Slug != args.Slug {
			continue
		}
		if args.Wipe {
			_ = os.RemoveAll(filepath.Join(r.Path, ".archigraph"))
		}
		if err := Index(r.Path, "", "", nil, false, false); err != nil {
			return rebuilt, "", fmt.Errorf("index %s: %w", r.Slug, err)
		}
		rebuilt = append(rebuilt, r.Slug)
	}
	// Cross-repo link passes run after every member is indexed.
	warning := ""
	if err := runLinksHook(args.Group); err != nil {
		// Best-effort — surface as a warning, not a hard failure.
		warning = fmt.Sprintf("link passes failed: %v", err)
	}
	return rebuilt, warning, nil
}

// mustEncodeStatus is a small helper for the `status` command when it
// prints the daemon's reply as JSON. Lives here so cmd/archigraph
// doesn't have to import encoding/json from a half-dozen call sites.
func mustEncodeStatus(w io.Writer, reply proto.StatusReply) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(reply)
}

// daemonNotRunningErr is the canonical user-facing error returned by
// any client subcommand when the daemon socket is unreachable.
var daemonNotRunningErr = errors.New(
	"daemon not running; run 'archigraph start' or reinstall via 'archigraph install'",
)
