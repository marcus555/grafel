/* ============================================================
   components/compound-topology/CompoundLens.tsx

   A SINGLE compound-topology canvas for ONE fixed `group_by` lens. Extracted
   from CompoundTopology (Model 1) so Model 2 (#4810, cross-linked lenses) can
   render two of them side-by-side and drive a SHARED selection across both.

   This is the Model-1 canvas (ELK/dagre layout, #4866 zones, #4887 handles)
   minus the group-by toggle — plus three Model-2 affordances:
     - `highlight`   — the cross-link maps (per-node / per-zone) to tint.
     - `selectedId`  — the node currently selected (anywhere).
     - `onSelectNode`— bubbles a node click up to the two-lens controller.

   The highlight tinting is injected into each node/zone's data so CTNode /
   CTZone render it; the layout (positions) is untouched.
   ============================================================ */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  type Node,
  type NodeMouseHandler,
  type NodeTypes,
  type EdgeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { cn } from "@/lib/utils";
import type { CompoundGroupBy } from "@/data/types";
import { useCompoundTopology } from "@/hooks/use-topology";
import {
  layoutCompoundTopology,
  layoutCompoundTopologyElk,
  defaultLayoutEngine,
  CT_NODE_TYPE,
  CT_ZONE_TYPE,
  CT_EDGE_TYPE,
  type CTLayoutResult,
} from "./layout";
import type { CrossLinkHighlight } from "./crossLink";
import { CTNode } from "./CTNode";
import { CTZone } from "./CTZone";
import { CTEdge } from "./CTEdge";

const nodeTypes: NodeTypes = {
  [CT_NODE_TYPE]: CTNode,
  [CT_ZONE_TYPE]: CTZone,
};
const edgeTypes: EdgeTypes = { [CT_EDGE_TYPE]: CTEdge };

export interface CompoundLensProps {
  groupId: string;
  groupBy: CompoundGroupBy;
  /** Cross-link highlight maps for THIS lens (Model 2). */
  highlight?: CrossLinkHighlight;
  /** Currently-selected node id (across both lenses). */
  selectedId?: string | null;
  /** Fired when a leaf node is clicked; null clears the selection. */
  onSelectNode?: (id: string | null) => void;
  className?: string;
}

function CompoundLensInner({
  groupId,
  groupBy,
  highlight,
  selectedId,
  onSelectNode,
  className,
}: CompoundLensProps) {
  // Per-lens collapse set (independent of the other lens).
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const onToggle = useCallback((zoneId: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(zoneId)) next.delete(zoneId);
      else next.add(zoneId);
      return next;
    });
  }, []);

  const { data, isLoading, isError } = useCompoundTopology(groupId, groupBy);

  const engine = useMemo(() => defaultLayoutEngine(), []);
  const EMPTY: CTLayoutResult = useMemo(
    () => ({ nodes: [], edges: [], capped: false, summaryEdgeCount: 0 }),
    [],
  );

  const dagreResult = useMemo(
    () => (engine === "dagre" ? layoutCompoundTopology(data, collapsed, onToggle) : EMPTY),
    [engine, data, collapsed, onToggle, EMPTY],
  );

  const [elkResult, setElkResult] = useState<CTLayoutResult>(EMPTY);
  const [elkLaidOut, setElkLaidOut] = useState(false);
  const elkRunId = useRef(0);
  useEffect(() => {
    if (engine !== "elk") return;
    const myRun = ++elkRunId.current;
    let cancelled = false;
    setElkLaidOut(false);
    layoutCompoundTopologyElk(data, collapsed, onToggle)
      .then((res) => {
        if (cancelled || myRun !== elkRunId.current) return;
        setElkResult(res);
        setElkLaidOut(true);
      })
      .catch(() => {
        if (cancelled || myRun !== elkRunId.current) return;
        setElkResult(layoutCompoundTopology(data, collapsed, onToggle));
        setElkLaidOut(true);
      });
    return () => {
      cancelled = true;
    };
  }, [engine, data, collapsed, onToggle]);

  const base = engine === "dagre" ? dagreResult : elkResult;
  const layingOut = engine === "elk" && !elkLaidOut;

  // Inject the cross-link highlight into each node/zone's data so CTNode/CTZone
  // render it. Positions are untouched — this is a cheap per-render remap.
  const nodes: Node[] = useMemo(() => {
    if (!highlight?.active) {
      // No selection — strip any stale highlight flags.
      return base.nodes.map((n) =>
        n.data && (n.data as { highlight?: unknown }).highlight !== undefined
          ? { ...n, data: { ...n.data, highlight: undefined, dimmed: false }, selected: false }
          : n,
      );
    }
    return base.nodes.map((n) => {
      const isZone = n.type === CT_ZONE_TYPE;
      const hl = isZone
        ? highlight.zones.get(n.id) ?? "none"
        : highlight.nodes.get(n.id) ?? "none";
      return {
        ...n,
        selected: !isZone && n.id === selectedId,
        data: { ...n.data, highlight: hl, dimmed: true },
      };
    });
  }, [base.nodes, highlight, selectedId]);

  const onNodeClick: NodeMouseHandler = useCallback(
    (_evt, node) => {
      if (node.type === CT_ZONE_TYPE) return; // zone clicks toggle collapse.
      if (!onSelectNode) return;
      onSelectNode(node.id === selectedId ? null : node.id);
    },
    [onSelectNode, selectedId],
  );

  const onPaneClick = useCallback(() => onSelectNode?.(null), [onSelectNode]);

  return (
    <div className={cn("relative h-full min-h-0", className)}>
      {isLoading || layingOut ? (
        <div className="flex h-full items-center justify-center text-sm text-text-4">
          {isLoading ? "Loading…" : "Laying out…"}
        </div>
      ) : isError ? (
        <div className="flex h-full flex-col items-center justify-center gap-1 text-center">
          <p className="text-md font-semibold text-text">Couldn&apos;t load topology</p>
          <p className="text-sm text-text-3">Make sure the daemon is running.</p>
        </div>
      ) : nodes.length === 0 ? (
        <div className="flex h-full items-center justify-center text-sm text-text-4">
          No architecturally-significant nodes.
        </div>
      ) : (
        <ReactFlow
          nodes={nodes}
          edges={base.edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          fitView
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          proOptions={{ hideAttribution: true }}
          minZoom={0.08}
          fitViewOptions={{ padding: 0.16 }}
        >
          <Background gap={18} size={1} color="var(--border)" />
          <Controls showInteractive={false} />
          <MiniMap pannable zoomable className="!border !border-border !bg-surface" />
        </ReactFlow>
      )}
    </div>
  );
}

/** A single compound-topology lens (one fixed group_by), wrapped in a provider. */
export function CompoundLens(props: CompoundLensProps) {
  return (
    <ReactFlowProvider>
      <CompoundLensInner {...props} />
    </ReactFlowProvider>
  );
}
