import { describe, it, expect } from "vitest";

import {
  anyRepoActive,
  engineStats,
  groupEnhancing,
  groupQueryable,
  joinIndexStatus,
  viewGraphEnabled,
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

describe("groupQueryable — graph is servable once every repo finished extraction", () => {
  it("false while ANY repo is still indexing (button stays disabled, no nav)", () => {
    expect(
      groupQueryable(
        status({
          repos: [
            { repo_slug: "a", indexing: false, enhancing: false, entities: 10, relationships: 5, graph_fb_mtime: 1 },
            { repo_slug: "b", indexing: true, enhancing: false, entities: 0, relationships: 0, graph_fb_mtime: 0 },
          ],
        }),
      ),
    ).toBe(false);
  });

  it("true once every repo has indexing === false (button enables)", () => {
    expect(
      groupQueryable(
        status({
          repos: [
            { repo_slug: "a", indexing: false, enhancing: false, entities: 10, relationships: 5, graph_fb_mtime: 1 },
            { repo_slug: "b", indexing: false, enhancing: false, entities: 8, relationships: 3, graph_fb_mtime: 2 },
          ],
        }),
      ),
    ).toBe(true);
  });

  it("true even while enhancing runs in the background (does NOT block on enhancing)", () => {
    expect(
      groupQueryable(
        status({
          repos: [{ repo_slug: "a", indexing: false, enhancing: true, entities: 10, relationships: 5, graph_fb_mtime: 1 }],
        }),
      ),
    ).toBe(true);
  });

  it("false when there are zero repos (nothing to visualize yet)", () => {
    expect(groupQueryable(status({ repos: [] }))).toBe(false);
  });

  it("false for undefined status (no status plane yet)", () => {
    expect(groupQueryable(undefined)).toBe(false);
  });
});

describe("viewGraphEnabled — 'View graph' button gate (Bug A)", () => {
  it("enables via the status plane when every repo has finished extraction", () => {
    const s = status({
      repos: [
        { repo_slug: "a", indexing: false, enhancing: false, entities: 10, relationships: 5, graph_fb_mtime: 1 },
      ],
    });
    // Job not done yet, but the status plane already reports queryable → enabled.
    expect(viewGraphEnabled(s, false, [row({ phase: "extracting_ast" })])).toBe(true);
  });

  it("FAST PATH: already-indexed 'up to date' group — status plane reports zero repos, but job done + all rows terminal → enabled", () => {
    // The primary bug: a rebuild that touches 0 repos leaves the status plane
    // empty, so groupQueryable stays false forever. The feed-terminal fallback
    // must still unlock the button.
    const emptyStatus = status({ repos: [] });
    expect(groupQueryable(emptyStatus)).toBe(false);
    expect(
      viewGraphEnabled(emptyStatus, true, [
        row({ repoSlug: "backend", phase: "done" }),
        row({ repoSlug: "frontend", phase: "done" }),
      ]),
    ).toBe(true);
  });

  it("FAST PATH works with NO status plane at all (undefined) once the job is done and rows are terminal", () => {
    expect(viewGraphEnabled(undefined, true, [row({ phase: "done" })])).toBe(true);
  });

  it("stays DISABLED during active indexing (job not done, a row still non-terminal, status not queryable)", () => {
    const s = status({
      repos: [
        { repo_slug: "a", indexing: false, enhancing: false, entities: 3, relationships: 0, graph_fb_mtime: 1 },
        { repo_slug: "b", indexing: true, enhancing: false, entities: 0, relationships: 0, graph_fb_mtime: 0 },
      ],
    });
    expect(
      viewGraphEnabled(s, false, [
        row({ repoSlug: "a", phase: "done" }),
        row({ repoSlug: "b", phase: "extracting_ast" }),
      ]),
    ).toBe(false);
  });

  it("stays DISABLED when the job finished but a row is still non-terminal (no premature unlock)", () => {
    expect(
      viewGraphEnabled(undefined, true, [
        row({ repoSlug: "a", phase: "done" }),
        row({ repoSlug: "b", phase: "materializing" }),
      ]),
    ).toBe(false);
  });

  it("stays DISABLED with zero rows even if the job is done (nothing to visualize)", () => {
    expect(viewGraphEnabled(undefined, true, [])).toBe(false);
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
