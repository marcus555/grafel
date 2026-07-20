/* ============================================================
   lib/group-unit.ts — landing group-card unit label.

   A group is either a MONOREPO (a single repo split into many
   module sub-paths, surfaced via `Group.monorepos`) or a plain
   MULTI-REPO group. The landing card shows a count stat that
   adapts to which one it is:

     • monorepo   → "Modules" + total module sub-path count
     • multi-repo → "Repos"   + top-level repo count

   Pure + dependency-free so it's unit-testable in isolation.
   ============================================================ */

import type { Group } from "@/data/types";

export interface GroupUnit {
  label: "Modules" | "Repos";
  count: number;
}

/**
 * Decide whether a group's count stat reads as "Modules" (monorepo) or
 * "Repos" (multi-repo), plus the matching count.
 *
 * A group counts as a monorepo when its `monorepos` map (parent-repo slug →
 * module sub-paths) is present and declares at least one module. In that case
 * the count is the TOTAL number of module sub-paths across every parent. Any
 * group without declared monorepo modules falls back to the top-level repo
 * count.
 */
export function groupUnitLabel(group: Pick<Group, "repos" | "monorepos">): GroupUnit {
  const monorepos = group.monorepos;
  if (monorepos) {
    const moduleCount = Object.values(monorepos).reduce(
      (sum, modules) => sum + modules.length,
      0,
    );
    if (moduleCount > 0) {
      return { label: "Modules", count: moduleCount };
    }
  }
  return { label: "Repos", count: group.repos.length };
}
