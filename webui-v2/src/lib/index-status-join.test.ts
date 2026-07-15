import { describe, it, expect } from "vitest";

import {
  anyRepoActive,
  engineStats,
  groupEnhancing,
  joinIndexStatus,
} from "./index-status-join";
import type { IndexStatusReply, ProgressRow } from "@/data/types";

function row(p: Partial<ProgressRow>): ProgressRow {
  return {
    key: p.repoSlug ?? "backend",
    repoSlug: "backend",
    phase: "done",
    filesDone: 0,
    filesTotal: 0,
    entitiesSoFar: 0,
    ts: 1,
    ...p,
  };
}

function status(p: Partial<IndexStatusReply>): IndexStatusReply {
  return {
    engine: { cpu_pct: 0, rss_mb: 0 },
    repos: [],
    ...p,
  };
}

describe("joinIndexStatus — join status-plane rows onto progress rows by repo_slug", () => {
  it("surfaces enhancing / indexing / entities / relationships onto the matching row", () => {
    const rows = [row({ repoSlug: "backend", phase: "done" })];
    const joined = joinIndexStatus(
      rows,
      status({
        repos: [
          {
            repo_slug: "backend",
            indexing: false,
            enhancing: true,
            entities: 3272,
            relationships: 8100,
            graph_fb_mtime: 123,
          },
        ],
      }),
    );
    expect(joined[0].enhancing).toBe(true);
    expect(joined[0].indexing).toBe(false);
    expect(joined[0].relationships).toBe(8100);
    // entities join backfills entitiesSoFar when the SSE count is behind.
    expect(joined[0].entitiesSoFar).toBe(3272);
  });

  it("does not clobber a higher SSE entity count with a lower status count", () => {
    const rows = [row({ repoSlug: "backend", entitiesSoFar: 5000 })];
    const joined = joinIndexStatus(
      rows,
      status({
        repos: [
          { repo_slug: "backend", indexing: false, enhancing: false, entities: 10, relationships: 0, graph_fb_mtime: 0 },
        ],
      }),
    );
    expect(joined[0].entitiesSoFar).toBe(5000);
  });

  it("leaves rows unchanged when there is no matching status repo", () => {
    const rows = [row({ repoSlug: "backend" })];
    const joined = joinIndexStatus(
      rows,
      status({ repos: [{ repo_slug: "other", indexing: false, enhancing: true, entities: 0, relationships: 0, graph_fb_mtime: 0 }] }),
    );
    expect(joined[0].enhancing).toBeUndefined();
  });

  it("is a no-op when status is undefined (returns the same rows)", () => {
    const rows = [row({ repoSlug: "backend" })];
    expect(joinIndexStatus(rows, undefined)).toBe(rows);
  });
});

describe("groupEnhancing — any repo enhancing", () => {
  it("true when at least one repo is enhancing", () => {
    expect(
      groupEnhancing(
        status({
          repos: [
            { repo_slug: "a", indexing: false, enhancing: false, entities: 0, relationships: 0, graph_fb_mtime: 0 },
            { repo_slug: "b", indexing: false, enhancing: true, entities: 0, relationships: 0, graph_fb_mtime: 0 },
          ],
        }),
      ),
    ).toBe(true);
  });

  it("false when no repo is enhancing", () => {
    expect(
      groupEnhancing(
        status({ repos: [{ repo_slug: "a", indexing: false, enhancing: false, entities: 0, relationships: 0, graph_fb_mtime: 0 }] }),
      ),
    ).toBe(false);
  });

  it("false for undefined status", () => {
    expect(groupEnhancing(undefined)).toBe(false);
  });
});

describe("anyRepoActive — poll-continuation predicate", () => {
  it("true while any repo is indexing OR enhancing", () => {
    expect(
      anyRepoActive(
        status({ repos: [{ repo_slug: "a", indexing: true, enhancing: false, entities: 0, relationships: 0, graph_fb_mtime: 0 }] }),
      ),
    ).toBe(true);
    expect(
      anyRepoActive(
        status({ repos: [{ repo_slug: "a", indexing: false, enhancing: true, entities: 0, relationships: 0, graph_fb_mtime: 0 }] }),
      ),
    ).toBe(true);
  });

  it("false once every repo is settled", () => {
    expect(
      anyRepoActive(
        status({ repos: [{ repo_slug: "a", indexing: false, enhancing: false, entities: 1, relationships: 1, graph_fb_mtime: 1 }] }),
      ),
    ).toBe(false);
  });

  it("false for undefined", () => {
    expect(anyRepoActive(undefined)).toBe(false);
  });
});

describe("engineStats — CPU/RSS passthrough", () => {
  it("returns the engine block", () => {
    expect(engineStats(status({ engine: { cpu_pct: 42.5, rss_mb: 512 } }))).toEqual({ cpu_pct: 42.5, rss_mb: 512 });
  });

  it("returns undefined for undefined status", () => {
    expect(engineStats(undefined)).toBeUndefined();
  });

  it("returns undefined for an all-zero engine block (nothing to show)", () => {
    expect(engineStats(status({ engine: { cpu_pct: 0, rss_mb: 0 } }))).toBeUndefined();
  });
});
