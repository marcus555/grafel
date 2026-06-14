/* ============================================================
   components/compound-topology/CrossLinkedTopology.tsx

   Model 2 of the compound-topology epic (#4810) — "cross-linked lenses".

   Two compound-topology canvases side-by-side:
     - LEFT  = Infra lens   (cloud → vpc/cluster → service, from IaC).
     - RIGHT = Code lens     (repo → module nesting).

   Both render the SAME entities (the compound payload returns one node set per
   group_by; only the zone hierarchy changes). Selecting a node in EITHER lens
   highlights its counterpart in the OTHER:
     - the SAME entity (identity cross-link — real, the node id is shared), and
     - any entity joined by a real typed edge (e.g. a service that READS a
       datastore), and
     - the containing zone boxes in the other lens, so you can see WHERE the
       counterpart lives in that hierarchy.

   The cross-link math is the pure, tested `computeCrossLink` helper. Styling
   matches Model 1 (#4866 zones, #4887 centered handles, ELK layout).
   ============================================================ */

import { useMemo, useState } from "react";
import { Cloud, Boxes, Link2, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useCompoundTopology } from "@/hooks/use-topology";
import { CompoundLens } from "./CompoundLens";
import { computeCrossLink } from "./crossLink";
import { useCoverageKind } from "@/hooks/use-coverage-kind";
import {
  CoverageKindOverlayToggle,
  coverageKindRingStyle,
} from "@/components/ui";

interface CrossLinkedTopologyProps {
  groupId: string;
  className?: string;
}

export function CrossLinkedTopology({ groupId, className }: CrossLinkedTopologyProps) {
  // Shared selection across both lenses (a node id, or null).
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // #5147 coverage-kind overlay — one toggle drives BOTH lenses' rings.
  const [coverageOverlay, setCoverageOverlay] = useState(false);
  const coverageKind = useCoverageKind(groupId);
  const coverageRing = useMemo(
    () => coverageKindRingStyle(coverageKind, coverageOverlay),
    [coverageKind, coverageOverlay],
  );

  // We need BOTH payloads here (not just inside each lens) to compute the
  // cross-link — these queries are shared/cached with the lens' own useQuery,
  // so this is not a duplicate fetch.
  const { data: infra } = useCompoundTopology(groupId, "infra");
  const { data: modules } = useCompoundTopology(groupId, "modules");

  const cross = useMemo(
    () => computeCrossLink(selectedId, infra, modules),
    [selectedId, infra, modules],
  );

  // Human-readable label for the selection banner.
  const selectedLabel = useMemo(() => {
    if (!selectedId) return null;
    const n =
      (infra?.nodes ?? []).find((x) => x.id === selectedId) ??
      (modules?.nodes ?? []).find((x) => x.id === selectedId);
    return n?.label ?? selectedId;
  }, [selectedId, infra, modules]);

  // Count of edge-linked counterparts (same in both lenses) for the banner.
  const linkedCount = useMemo(() => {
    if (!selectedId) return 0;
    let c = 0;
    for (const hl of cross.infra.nodes.values()) if (hl === "linked") c++;
    return c;
  }, [cross, selectedId]);

  return (
    <div className={cn("flex h-full min-h-0 flex-col", className)}>
      {/* Cross-link banner */}
      <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface px-3 py-2 text-xs">
        <Link2 size={13} className="text-accent" />
        <span className="font-medium text-text-2">Cross-linked lenses</span>
        <span className="text-text-4">
          select a node in either lens to highlight its counterpart in the other
        </span>
        {/* #5147 coverage-kind overlay toggle + legend (drives both lenses) */}
        <CoverageKindOverlayToggle
          state={coverageKind}
          enabled={coverageOverlay}
          onToggle={() => setCoverageOverlay((v) => !v)}
        />
        {selectedId && (
          <div className="ml-auto flex items-center gap-2">
            <span className="rounded-full bg-accent/15 px-2 py-0.5 text-accent">
              {selectedLabel}
            </span>
            <span className="text-text-4 tabular-nums">
              {linkedCount} edge-linked{linkedCount === 1 ? " counterpart" : " counterparts"}
            </span>
            <button
              type="button"
              onClick={() => setSelectedId(null)}
              className="inline-flex items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-text-3 hover:bg-surface-2"
              title="Clear selection"
            >
              <X size={11} /> clear
            </button>
          </div>
        )}
      </div>

      {/* Two lenses */}
      <div className="flex min-h-0 flex-1 divide-x divide-border">
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-1.5 border-b border-border bg-surface-2 px-3 py-1.5 text-[11px] font-medium text-text-2">
            <Cloud size={12} className="text-info" /> Infra
            <span className="text-text-4">cloud · network · service</span>
          </div>
          <CompoundLens
            groupId={groupId}
            groupBy="infra"
            highlight={cross.infra}
            selectedId={selectedId}
            onSelectNode={setSelectedId}
            coverageRing={coverageRing}
            className="flex-1"
          />
        </div>
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-1.5 border-b border-border bg-surface-2 px-3 py-1.5 text-[11px] font-medium text-text-2">
            <Boxes size={12} className="text-accent" /> Code / Modules
            <span className="text-text-4">repo · module</span>
          </div>
          <CompoundLens
            groupId={groupId}
            groupBy="modules"
            highlight={cross.modules}
            selectedId={selectedId}
            onSelectNode={setSelectedId}
            coverageRing={coverageRing}
            className="flex-1"
          />
        </div>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-3 border-t border-border bg-surface px-3 py-1.5 text-[10px] text-text-4">
        <span className="inline-flex items-center gap-1">
          <span className="inline-block size-2.5 rounded-sm border-2" style={{ borderColor: "var(--accent)" }} />
          same entity (identity cross-link)
        </span>
        <span className="inline-flex items-center gap-1">
          <span className="inline-block size-2.5 rounded-sm border-2" style={{ borderColor: "var(--info)" }} />
          linked by a typed edge (reads/writes/invokes…)
        </span>
        <span className="text-text-4">
          links are real graph data (shared node identity + typed edges) — none inferred
        </span>
      </div>
    </div>
  );
}
