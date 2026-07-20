import { describe, it, expect } from "vitest";

import { rowMetricTail } from "./index-progress-row-metrics";
import type { ProgressRow } from "@/data/types";

function row(p: Partial<ProgressRow>): ProgressRow {
  return {
    key: "backend",
    repoSlug: "backend",
    phase: "extracting_ast",
    filesDone: 0,
    filesTotal: 0,
    entitiesSoFar: 0,
    ts: 1,
    ...p,
  };
}

describe("rowMetricTail — TUI-parity row tail (Bug B)", () => {
  it("leads with 'X/Y files' while indexing, matching the TUI", () => {
    const tail = rowMetricTail(row({ phase: "extracting_ast", filesDone: 160, filesTotal: 173 }));
    expect(tail).toBe("160/173 files");
  });

  it("renders files · entities · rels in the TUI's order", () => {
    const tail = rowMetricTail(
      row({ phase: "extracting_ast", filesDone: 160, filesTotal: 173, entitiesSoFar: 166, relationships: 201 }),
    );
    expect(tail).toBe("160/173 files · 166 entities · 201 rels");
  });

  it("hides files once the row is terminal (mirrors the TUI's !Terminal() guard) but keeps entities/rels", () => {
    const tail = rowMetricTail(
      row({ phase: "done", filesDone: 173, filesTotal: 173, entitiesSoFar: 166, relationships: 201 }),
    );
    expect(tail).toBe("166 entities · 201 rels");
  });

  it("omits files when the total is unknown (0)", () => {
    const tail = rowMetricTail(row({ phase: "scanning", filesDone: 0, filesTotal: 0, entitiesSoFar: 12 }));
    expect(tail).toBe("12 entities");
  });

  it("omits rels when there is no status-plane relationship count", () => {
    const tail = rowMetricTail(row({ phase: "extracting_ast", filesDone: 5, filesTotal: 10, entitiesSoFar: 3 }));
    expect(tail).toBe("5/10 files · 3 entities");
  });

  it("returns an empty string when there is nothing to show", () => {
    expect(rowMetricTail(row({ phase: "done", filesTotal: 0, entitiesSoFar: 0 }))).toBe("");
  });
});
