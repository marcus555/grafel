/**
 * Branch-logic tests for the #5147 coverage-kind NODE overlay — the diagram
 * node decoration that extends the #5067 row chip to the flow-dag / topology /
 * iac surfaces.
 *
 * Two pure units are covered:
 *   - resolveCoverageKindNodeDecoration — kind→ring-color projection over the
 *     shared resolver (same precedence/tones as the row chip).
 *   - coverageKindRingStyle — the inline boxShadow the surfaces stamp onto a
 *     node, including the load-bearing "degrade to NO ring" behaviour so a node
 *     with no real coverage signal is never given a fake authoritative tint.
 *
 * Pure, so it runs under the default (node) vitest environment with no DOM.
 *
 * Run with: npx vitest run src/lib/coverage-kind-node-overlay.test.ts
 */
import { describe, it, expect } from "vitest";
import {
  resolveCoverageKindNodeDecoration,
  type CoverageSourceState,
} from "./coverage-provenance";
import { coverageKindRingStyle } from "@/components/ui/coverage-kind-overlay";
import type { CoverageKindState } from "@/hooks/use-coverage-kind";

function stateOf(src: CoverageSourceState | null): CoverageKindState {
  return {
    state: src ?? {},
    decoration: resolveCoverageKindNodeDecoration(src),
    ready: true,
  };
}

describe("resolveCoverageKindNodeDecoration — ring color per kind", () => {
  it("LINE ⇒ success ring, authoritative", () => {
    const d = resolveCoverageKindNodeDecoration({ line: { source: "lcov" } });
    expect(d.kind).toBe("line");
    expect(d.ringColor).toBe("var(--success)");
    expect(d.authoritative).toBe(true);
    expect(d.short).toBe("Line");
  });

  it("REACH ⇒ info ring, not authoritative", () => {
    const d = resolveCoverageKindNodeDecoration({ reachabilityAvailable: true });
    expect(d.kind).toBe("reachability");
    expect(d.ringColor).toBe("var(--info)");
    expect(d.authoritative).toBe(false);
  });

  it("CAPABILITY (nothing wired / null) ⇒ neutral ring, not authoritative", () => {
    for (const s of [{}, null] as const) {
      const d = resolveCoverageKindNodeDecoration(s);
      expect(d.kind).toBe("capability");
      expect(d.ringColor).toBe("var(--text-4)");
      expect(d.authoritative).toBe(false);
    }
  });

  it("precedence matches the row chip: line ▸ reach ▸ capability", () => {
    expect(
      resolveCoverageKindNodeDecoration({
        line: { source: "jacoco" },
        reachabilityAvailable: true,
      }).kind,
    ).toBe("line");
  });
});

describe("coverageKindRingStyle — node boxShadow", () => {
  it("disabled ⇒ no ring (preserves any existing shadow)", () => {
    const st = stateOf({ line: { source: "lcov" } });
    expect(coverageKindRingStyle(st, false)).toEqual({});
    expect(coverageKindRingStyle(st, false, "0 0 0 2px var(--accent)")).toEqual({
      boxShadow: "0 0 0 2px var(--accent)",
    });
  });

  it("enabled + capability (no real signal) ⇒ NO ring — never a fake green", () => {
    const st = stateOf({}); // ⇒ capability default
    expect(coverageKindRingStyle(st, true)).toEqual({});
    // an existing selection shadow is preserved, just not augmented.
    expect(coverageKindRingStyle(st, true, "shadow")).toEqual({
      boxShadow: "shadow",
    });
  });

  it("enabled + line ⇒ success ring", () => {
    const st = stateOf({ line: { source: "lcov" } });
    expect(coverageKindRingStyle(st, true)).toEqual({
      boxShadow: "0 0 0 2px var(--success)",
    });
  });

  it("enabled + reach ⇒ info ring, composed AFTER an existing shadow", () => {
    const st = stateOf({ reachabilityAvailable: true });
    expect(coverageKindRingStyle(st, true, "0 0 0 2px var(--accent)")).toEqual({
      boxShadow: "0 0 0 2px var(--accent), 0 0 0 2px var(--info)",
    });
  });
});
