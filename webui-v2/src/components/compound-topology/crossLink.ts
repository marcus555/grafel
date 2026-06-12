/* ============================================================
   components/compound-topology/crossLink.ts

   Model 2 of the compound-topology epic (#4810) — "cross-linked lenses".

   The compound payload (topology_compound.go) returns the SAME node set for
   every `group_by`; only each node's `zone_path` changes per lens. That makes
   the cross-link between the infra lens and the code/modules lens REAL, not
   inferred: a node selected in one lens is the *same entity* in the other —
   we just light up where it lives in the other hierarchy.

   On top of that identity link we add a second REAL link: the typed edges
   (reads/writes/invokes/consumes/routes/depends) that already join code
   entities to the IaC resources (datastores/queues/functions) they use. So
   selecting a code module also highlights the infra resources it talks to, and
   selecting an infra resource highlights the code that uses it.

   This module is pure (no React / DOM) so the mapping is unit-testable.
   ============================================================ */

import type {
  CompoundTopologyResponse,
  CompoundNode,
  CompoundEdge,
} from "@/data/types";
import type { CTHighlight } from "./layout";

/** The cross-link result for ONE lens, keyed by node id and zone id. */
export interface CrossLinkHighlight {
  /** Per-node highlight state for this lens. */
  nodes: Map<string, CTHighlight>;
  /** Per-zone highlight state for this lens (a zone that contains a counterpart). */
  zones: Map<string, CTHighlight>;
  /** True when there is anything to highlight (a selection is active). */
  active: boolean;
}

const EMPTY: CrossLinkHighlight = {
  nodes: new Map(),
  zones: new Map(),
  active: false,
};

/** A node id that exists in BOTH payloads is a real identity cross-link. */
function indexNodeIds(data: CompoundTopologyResponse | undefined): Set<string> {
  const s = new Set<string>();
  for (const n of data?.nodes ?? []) s.add(n.id);
  return s;
}

/**
 * neighborsOf returns the set of node ids joined to `id` by any typed edge in
 * `edges`, in either direction. These are REAL relationship edges (e.g. a
 * service that READS a datastore) — the cross-lens "linked" counterparts.
 */
function neighborsOf(id: string, edges: CompoundEdge[]): Set<string> {
  const out = new Set<string>();
  for (const e of edges) {
    if (e.source === id) out.add(e.target);
    else if (e.target === id) out.add(e.source);
  }
  return out;
}

/**
 * Promote each highlighted node's zone ancestors so the containing boxes in the
 * other lens light up too — that is the whole point of Model 2: see WHERE the
 * counterpart sits in the other hierarchy. "primary" wins over "linked".
 */
function highlightZones(
  nodeHL: Map<string, CTHighlight>,
  nodeIndex: Map<string, CompoundNode>,
): Map<string, CTHighlight> {
  const zones = new Map<string, CTHighlight>();
  for (const [nodeId, hl] of nodeHL) {
    if (hl === "none") continue;
    const n = nodeIndex.get(nodeId);
    if (!n) continue;
    for (const zid of n.zone_path) {
      const cur = zones.get(zid);
      // primary outranks linked.
      if (cur === "primary") continue;
      zones.set(zid, hl === "primary" ? "primary" : "linked");
    }
  }
  return zones;
}

/**
 * computeCrossLink builds the highlight maps for BOTH lenses given the node
 * selected in `selectedLens`.
 *
 *   - In the SELECTED lens: the selected node is "primary"; its edge-neighbors
 *     are "linked"; everything else "none".
 *   - In the OTHER lens: the same node id (if present — it almost always is,
 *     since both lenses share the node set) is "primary"; its edge-neighbors
 *     are "linked"; everything else "none". Zone ancestors of any highlighted
 *     node are lit so the containing boxes read.
 *
 * Returns `{ infra, modules }`. When nothing is selected both are EMPTY.
 */
export function computeCrossLink(
  selectedId: string | null,
  infra: CompoundTopologyResponse | undefined,
  modules: CompoundTopologyResponse | undefined,
): { infra: CrossLinkHighlight; modules: CrossLinkHighlight } {
  if (!selectedId) return { infra: EMPTY, modules: EMPTY };

  const infraIds = indexNodeIds(infra);
  const moduleIds = indexNodeIds(modules);
  // The selected id must exist in at least one lens to be meaningful.
  if (!infraIds.has(selectedId) && !moduleIds.has(selectedId)) {
    return { infra: EMPTY, modules: EMPTY };
  }

  // Edge-neighbors are identical across lenses (edges don't change per lens),
  // so compute once from whichever payload is present.
  const edges = infra?.edges ?? modules?.edges ?? [];
  const neighbors = neighborsOf(selectedId, edges);

  function forLens(
    data: CompoundTopologyResponse | undefined,
    ids: Set<string>,
  ): CrossLinkHighlight {
    const nodes = new Map<string, CTHighlight>();
    for (const id of ids) {
      if (id === selectedId) nodes.set(id, "primary");
      else if (neighbors.has(id)) nodes.set(id, "linked");
      else nodes.set(id, "none");
    }
    const nodeIndex = new Map<string, CompoundNode>(
      (data?.nodes ?? []).map((n) => [n.id, n] as const),
    );
    const zones = highlightZones(nodes, nodeIndex);
    return { nodes, zones, active: true };
  }

  return {
    infra: forLens(infra, infraIds),
    modules: forLens(modules, moduleIds),
  };
}
