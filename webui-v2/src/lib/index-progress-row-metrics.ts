/* ============================================================
   index-progress-row-metrics.ts — the per-repo / per-module row metric tail
   (Bug B: web ↔ TUI parity).

   The TUI renders each index row as `name · phase · files done/total · entities`
   (internal/cli/wiztui/indexview.go renderRow), leading with the files count. The
   web row historically showed only "· N entities · M rels" (no files), because it
   dropped the leading files segment. This pure helper reproduces the TUI's tail
   verbatim so the web shows the SAME per-module information as the TUI.

   Extracted here (not inline in the .tsx) so it is unit-testable under
   `vitest run src/lib` without a DOM — mirroring index-progress-fold.ts.
   ============================================================ */

import type { ProgressRow } from "@/data/types";

/**
 * The right-aligned metric tail for one progress row, mirroring the TUI's
 * renderRow: `files done/total · entities · rels`, joined by " · ".
 *
 * Files LEAD (as in the TUI) and — again matching the TUI (`FilesTotal > 0 &&
 * !Terminal()`) — are shown only while the row is NON-terminal and has a known
 * total, since a finished row's file count is no longer informative. Entities and
 * relationships follow whenever they are positive. `relationships` is only ever
 * present via the status-plane join (the SSE feed carries entities only), so it
 * is optional. Returns "" when there is nothing to show.
 */
export function rowMetricTail(
  row: Pick<ProgressRow, "phase" | "filesDone" | "filesTotal" | "entitiesSoFar" | "relationships">,
): string {
  const terminal = row.phase === "done" || row.phase === "error";
  const parts: string[] = [];
  if (row.filesTotal > 0 && !terminal) {
    parts.push(`${row.filesDone}/${row.filesTotal} files`);
  }
  if (row.entitiesSoFar > 0) {
    parts.push(`${row.entitiesSoFar} entities`);
  }
  if (row.relationships != null && row.relationships > 0) {
    parts.push(`${row.relationships} rels`);
  }
  return parts.join(" · ");
}
