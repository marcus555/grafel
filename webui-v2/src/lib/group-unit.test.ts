import { describe, it, expect } from "vitest";
import { groupUnitLabel } from "./group-unit";

describe("groupUnitLabel", () => {
  it("labels a monorepo group 'Modules' with the total sub-path count", () => {
    // corpus-style: one repo split into 16 module sub-paths.
    const group = {
      repos: ["corpus"],
      monorepos: {
        corpus: Array.from({ length: 16 }, (_, i) => `packages/pkg-${i}`),
      },
    };
    expect(groupUnitLabel(group)).toEqual({ label: "Modules", count: 16 });
  });

  it("sums module sub-paths across multiple monorepo parents", () => {
    const group = {
      repos: ["a", "b"],
      monorepos: {
        a: ["packages/x", "packages/y"],
        b: ["libs/z"],
      },
    };
    expect(groupUnitLabel(group)).toEqual({ label: "Modules", count: 3 });
  });

  it("labels a multi-repo group 'Repos' with the repo count", () => {
    // upvate-style: 3 independent repos, no monorepo modules.
    const group = { repos: ["api", "web", "worker"] };
    expect(groupUnitLabel(group)).toEqual({ label: "Repos", count: 3 });
  });

  it("falls back to 'Repos' when the monorepos map is present but empty", () => {
    const group = { repos: ["solo"], monorepos: {} };
    expect(groupUnitLabel(group)).toEqual({ label: "Repos", count: 1 });
  });

  it("falls back to 'Repos' when every monorepo parent declares zero modules", () => {
    const group = { repos: ["solo"], monorepos: { solo: [] } };
    expect(groupUnitLabel(group)).toEqual({ label: "Repos", count: 1 });
  });

  it("handles an empty group (no repos, no monorepos)", () => {
    const group = { repos: [] };
    expect(groupUnitLabel(group)).toEqual({ label: "Repos", count: 0 });
  });
});
