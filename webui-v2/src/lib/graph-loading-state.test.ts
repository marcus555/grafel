/* ============================================================
   lib/graph-loading-state.test.ts — the pure loading-state derivation for
   the Graph screen (#48). The screen must NEVER sit at a blank "loading"
   forever: isLoading clears as soon as ANYTHING is renderable (first
   streamed nodes, or the fallback blob's first data), and a cold group
   shows a "warming" affordance instead of a dead blank.
   ============================================================ */

import { describe, expect, it } from "vitest";
import { deriveGraphLoading } from "./graph-loading-state";

const base = {
  streamPhase: "warming" as const,
  streamHasMeta: false,
  streamNodeCount: 0,
  fallbackActive: false,
  fallbackIsLoading: false,
  fallbackNodeCount: 0,
};

describe("deriveGraphLoading — stream path", () => {
  it("is loading + warming before meta on a cold group", () => {
    const s = deriveGraphLoading({ ...base, streamPhase: "warming" });
    expect(s.isLoading).toBe(true);
    expect(s.isWarming).toBe(true);
    expect(s.showProgress).toBe(true);
  });

  it("clears isLoading as soon as the FIRST streamed nodes land", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "streaming",
      streamHasMeta: true,
      streamNodeCount: 1,
    });
    expect(s.isLoading).toBe(false);
    expect(s.isWarming).toBe(false);
    // Still progressing — the progress affordance stays until done.
    expect(s.showProgress).toBe(true);
  });

  it("stays loading when meta has landed but no nodes yet", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "streaming",
      streamHasMeta: true,
      streamNodeCount: 0,
    });
    expect(s.isLoading).toBe(true);
  });

  it("is not loading once done with nodes", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "done",
      streamHasMeta: true,
      streamNodeCount: 42,
    });
    expect(s.isLoading).toBe(false);
    expect(s.showProgress).toBe(false);
  });
});

describe("deriveGraphLoading — fallback path (blob)", () => {
  it("is loading while the fallback blob is in flight with no data", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "error",
      fallbackActive: true,
      fallbackIsLoading: true,
      fallbackNodeCount: 0,
    });
    expect(s.isLoading).toBe(true);
    // Not "warming" — that affordance is stream-only.
    expect(s.isWarming).toBe(false);
  });

  it("clears isLoading as soon as the fallback has ANY renderable data, even if react-query still reports loading", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "error",
      fallbackActive: true,
      fallbackIsLoading: true,
      fallbackNodeCount: 5,
    });
    expect(s.isLoading).toBe(false);
  });

  it("clears isLoading when the fallback settles with data", () => {
    const s = deriveGraphLoading({
      ...base,
      streamPhase: "error",
      fallbackActive: true,
      fallbackIsLoading: false,
      fallbackNodeCount: 100,
    });
    expect(s.isLoading).toBe(false);
  });
});
