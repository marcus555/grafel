/* ============================================================
   IndexProgressFeed — per-repo / per-MODULE live indexing rows (#1527, nested
   monorepo modules #47 phase 2).

   Renders ONE row per repo. For a monorepo, the sibling package rows are nested
   UNDER a parent header row (indented children) — mirroring the TUI — when the
   caller passes pre-computed `groups` (via nestRows). When only `rows` are
   passed (e.g. the Operations activity feed) it renders the flat list, exactly
   as before.

   Each row shows its phase, a files-done/total bar, entity count, and the file
   currently being processed. A row that is `enhancing` (graph queryable,
   background enrichment still running) shows an "enhancing" badge — it is
   success, never "Failed".
   ============================================================ */

import { CheckCircle2, AlertTriangle, Loader2, Sparkles } from "lucide-react";

import { Badge } from "@/components/ui";
import { cn } from "@/lib/utils";
import type { ProgressGroup, ProgressPhase, ProgressRow } from "@/data/types";

const PHASE_LABEL: Record<ProgressPhase, string> = {
  scanning: "Scanning files",
  extracting_ast: "Extracting AST",
  resolving_refs: "Resolving refs",
  running_algorithms: "Running algorithms",
  materializing: "Materializing",
  // #5334 — granular graph-assembly phases.
  building_communities: "Building communities",
  computing_centrality: "Computing centrality",
  detecting_links: "Detecting cross-repo links",
  computing_flows: "Computing flows",
  writing_graph: "Writing graph",
  done: "Indexed",
  error: "Failed",
};

function pct(row: ProgressRow): number {
  if (row.phase === "done") return 100;
  if (row.filesTotal <= 0) return row.phase === "scanning" ? 5 : 10;
  return Math.min(99, Math.round((row.filesDone / row.filesTotal) * 100));
}

function PhaseIcon({ phase }: { phase: ProgressPhase }) {
  if (phase === "done") return <CheckCircle2 size={13} className="text-success" />;
  if (phase === "error") return <AlertTriangle size={13} className="text-danger" />;
  return <Loader2 size={13} className="animate-spin text-accent-strong" />;
}

/** One progress row body (used both flat and as a nested monorepo child). */
function ProgressRowItem({ row, nested }: { row: ProgressRow; nested?: boolean }) {
  const p = pct(row);
  const failed = row.phase === "error";
  const done = row.phase === "done";
  const label = row.module ?? row.repoSlug;
  return (
    <li
      className={cn(
        "rounded-lg border border-border bg-surface p-2.5",
        nested && "ml-3 border-l-2 border-l-border/70",
      )}
      data-testid="progress-row"
      data-module={row.module ?? ""}
      data-repo={row.repoSlug}
      data-nested={nested ? "true" : "false"}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <PhaseIcon phase={row.phase} />
          <span className="truncate font-mono text-xs text-text-2" title={label}>
            {label}
          </span>
          {row.module && (
            <Badge tone="info" className="shrink-0">
              module
            </Badge>
          )}
          {row.enhancing && (
            <Badge tone="accent" className="shrink-0" data-testid="row-enhancing-badge">
              <Sparkles size={10} className="mr-0.5" /> enhancing
            </Badge>
          )}
        </div>
        <span className="shrink-0 text-[11px] text-text-4">{PHASE_LABEL[row.phase]}</span>
      </div>

      <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-300",
            failed ? "bg-danger" : done ? "bg-success" : "bg-accent",
          )}
          style={{ width: `${p}%` }}
        />
      </div>

      <div className="mt-1 flex items-center justify-between gap-2 text-[11px] text-text-4">
        <span className="truncate font-mono" title={row.currentFile}>
          {failed
            ? row.error || "error"
            : row.currentFile || (row.filesTotal > 0 ? `${row.filesDone}/${row.filesTotal} files` : "")}
        </span>
        <span className="shrink-0 tabular-nums">
          {row.filesTotal > 0 && `${row.filesDone}/${row.filesTotal}`}
          {row.entitiesSoFar > 0 && ` · ${row.entitiesSoFar} entities`}
          {row.relationships != null && row.relationships > 0 && ` · ${row.relationships} rels`}
        </span>
      </div>
    </li>
  );
}

/** Monorepo parent header + its indented per-module children. */
function MonorepoGroupItem({ group }: { group: ProgressGroup }) {
  return (
    <li
      className="rounded-lg border border-border bg-surface-2/40 p-2"
      data-testid="progress-group"
      data-kind="monorepo"
      data-group={group.label}
    >
      <div className="flex items-center justify-between gap-2 px-1 pb-1.5">
        <div className="flex min-w-0 items-center gap-2">
          <PhaseIcon phase={group.phase} />
          <span className="truncate font-mono text-xs text-text-1" title={group.label}>
            {group.label}
          </span>
          <Badge tone="neutral" className="shrink-0">
            monorepo · {group.children.length}
          </Badge>
          {group.enhancing && (
            <Badge tone="accent" className="shrink-0">
              <Sparkles size={10} className="mr-0.5" /> enhancing
            </Badge>
          )}
        </div>
        <span className="shrink-0 text-[11px] text-text-4">{PHASE_LABEL[group.phase]}</span>
      </div>
      <ul className="space-y-1.5">
        {group.children.map((child) => (
          <ProgressRowItem key={child.key} row={child} nested />
        ))}
      </ul>
    </li>
  );
}

export interface IndexProgressFeedProps {
  /** Flat per-repo rows (Operations activity feed, and the fallback path). */
  rows: ProgressRow[];
  /**
   * Pre-nested groups (via nestRows) — when provided, the feed renders monorepo
   * modules nested under a parent header and standalone repos flat. Falls back
   * to the flat `rows` list when absent.
   */
  groups?: ProgressGroup[];
  /** Shown while connected but no events have arrived yet. */
  loading?: boolean;
  className?: string;
}

export function IndexProgressFeed({ rows, groups, loading, className }: IndexProgressFeedProps) {
  const empty = groups ? groups.length === 0 : rows.length === 0;
  if (empty) {
    return (
      <p className={cn("text-xs text-text-4", className)} data-testid="progress-feed-empty">
        {loading ? "Waiting for the first files…" : "No per-repo progress yet."}
      </p>
    );
  }

  return (
    <ul className={cn("space-y-2", className)} data-testid="progress-feed">
      {groups
        ? groups.map((g) =>
            g.kind === "monorepo" ? (
              <MonorepoGroupItem key={g.key} group={g} />
            ) : (
              <ProgressRowItem key={g.key} row={g.row!} />
            ),
          )
        : rows.map((row) => <ProgressRowItem key={row.key} row={row} />)}
    </ul>
  );
}
