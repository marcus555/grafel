/* crossLink.test.ts — Model 2 (#4810) cross-lens highlight mapping.

   Verifies the identity cross-link (same node id across the infra/modules
   lenses), the typed-edge "linked" counterparts, and zone-ancestor promotion
   in each lens. The infra and modules payloads share the SAME node ids — only
   the zone_path differs per lens — which is exactly the real data the live
   compound endpoint returns. */

import { describe, it, expect } from "vitest";
import { computeCrossLink } from "./crossLink";
import type { CompoundTopologyResponse } from "@/data/types";

// Shared edges (identical across lenses): svc reads db, ep invokes svc.
const EDGES: CompoundTopologyResponse["edges"] = [
  { source: "api::svc", target: "api::db", type: "reads", label: "reads", agg_key: "reads api::svc api::db" },
  { source: "api::ep", target: "api::svc", type: "invokes", label: "invokes", agg_key: "invokes api::ep api::svc" },
];

// INFRA lens: db sits inside a cloud→service zone; svc/ep fall back to repo.
function infra(): CompoundTopologyResponse {
  return {
    group_by: "infra",
    tiers: ["client", "edge", "auth", "compute", "data", "messaging", "external"],
    zones: [
      { id: "cloud:aws", label: "aws", kind: "cloud", node_count: 1 },
      { id: "infra:aws:data", label: "data", parent_id: "cloud:aws", kind: "service", node_count: 1 },
      { id: "repo:api", label: "api", kind: "repo", node_count: 2 },
    ],
    nodes: [
      { id: "api::db", label: "db", kind: "Datastore", tier: "data", repo: "api", zone_path: ["cloud:aws", "infra:aws:data"] },
      { id: "api::svc", label: "Svc", kind: "Service", tier: "compute", repo: "api", zone_path: ["repo:api"] },
      { id: "api::ep", label: "GET /x", kind: "HTTPEndpoint", tier: "edge", repo: "api", zone_path: ["repo:api"] },
    ],
    edges: EDGES,
  };
}

// MODULES lens: SAME node ids, nested by code path instead.
function modules(): CompoundTopologyResponse {
  return {
    group_by: "modules",
    tiers: ["client", "edge", "auth", "compute", "data", "messaging", "external"],
    zones: [
      { id: "repo:api", label: "api", kind: "repo", node_count: 3 },
      { id: "mod:api/svc", label: "svc", parent_id: "repo:api", kind: "module", node_count: 2 },
    ],
    nodes: [
      { id: "api::db", label: "db", kind: "Datastore", tier: "data", repo: "api", zone_path: ["repo:api", "mod:api/svc"] },
      { id: "api::svc", label: "Svc", kind: "Service", tier: "compute", repo: "api", zone_path: ["repo:api", "mod:api/svc"] },
      { id: "api::ep", label: "GET /x", kind: "HTTPEndpoint", tier: "edge", repo: "api", zone_path: ["repo:api"] },
    ],
    edges: EDGES,
  };
}

describe("computeCrossLink", () => {
  it("returns inactive maps when nothing is selected", () => {
    const { infra: i, modules: m } = computeCrossLink(null, infra(), modules());
    expect(i.active).toBe(false);
    expect(m.active).toBe(false);
    expect(i.nodes.size).toBe(0);
  });

  it("marks the SAME entity primary in BOTH lenses (identity cross-link)", () => {
    const { infra: i, modules: m } = computeCrossLink("api::db", infra(), modules());
    expect(i.active).toBe(true);
    expect(i.nodes.get("api::db")).toBe("primary");
    expect(m.nodes.get("api::db")).toBe("primary"); // same id, other lens
  });

  it("marks typed-edge neighbors as linked in both lenses", () => {
    // db is read BY svc → selecting db links svc (and only svc, not ep).
    const { infra: i, modules: m } = computeCrossLink("api::db", infra(), modules());
    expect(i.nodes.get("api::svc")).toBe("linked");
    expect(m.nodes.get("api::svc")).toBe("linked");
    expect(i.nodes.get("api::ep")).toBe("none");
  });

  it("selecting a code node links the infra resources it uses", () => {
    // svc reads db AND is invoked by ep → both are linked.
    const { infra: i } = computeCrossLink("api::svc", infra(), modules());
    expect(i.nodes.get("api::svc")).toBe("primary");
    expect(i.nodes.get("api::db")).toBe("linked");
    expect(i.nodes.get("api::ep")).toBe("linked");
  });

  it("promotes zone ancestors of highlighted nodes (shows WHERE the counterpart lives)", () => {
    const { infra: i, modules: m } = computeCrossLink("api::db", infra(), modules());
    // INFRA: db lives in cloud:aws → infra:aws:data, both should be primary.
    expect(i.zones.get("cloud:aws")).toBe("primary");
    expect(i.zones.get("infra:aws:data")).toBe("primary");
    // MODULES: db lives in repo:api → mod:api/svc.
    expect(m.zones.get("repo:api")).toBe("primary");
    expect(m.zones.get("mod:api/svc")).toBe("primary");
  });

  it("primary outranks linked when a zone holds both", () => {
    // In modules, repo:api holds db(primary) + svc(linked) → primary wins.
    const { modules: m } = computeCrossLink("api::db", infra(), modules());
    expect(m.zones.get("repo:api")).toBe("primary");
  });

  it("ignores a selection id absent from both payloads", () => {
    const { infra: i, modules: m } = computeCrossLink("ghost::x", infra(), modules());
    expect(i.active).toBe(false);
    expect(m.active).toBe(false);
  });
});
