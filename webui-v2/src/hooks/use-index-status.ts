/* ============================================================
   hooks/use-index-status.ts — poll the index status plane (#47 phase 2).

   The SSE progress feed (use-index-progress) drives per-repo phase/file
   progress but goes quiet once the graph is queryable — it can't see the
   `enhancing` (background relationship enrichment) tail or engine CPU/RSS. This
   hook polls GET /api/v2/groups/:group/index-status alongside it, keyed by
   group, so the wizard can join the status-plane rows onto the SSE rows by
   repo_slug, drive a secondary "enhancing" bar, and show CPU/RAM badges —
   parity with the TUI (internal/cli/wiztui/indexview.go bgProgressBlock).

   Polling: every ~1.5s while `enabled` and at least one repo is still
   indexing/enhancing; it keeps polling one extra beat after the SSE feed goes
   terminal so the enhancing tail is observed, then stops once every repo has
   settled. Mirrors the proven refetchInterval pattern of useWizardJob.
   ============================================================ */

import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api";

/** Poll interval (ms) while the group is still indexing/enhancing. */
const POLL_MS = 1500;

/**
 * Poll the index status plane for `group`. Pass `enabled === false` (or a falsy
 * group) to stay idle — e.g. before the wizard has a target group, or after it
 * closes. Returns the standard react-query result; read `data` for the
 * {engine, repos} payload.
 */
export function useIndexStatus(group: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ["index-status", group],
    queryFn: () => api.getIndexStatus(group as string),
    enabled: enabled && !!group,
    // Poll steadily while enabled. The wizard gates `enabled` on the index step,
    // so polling naturally spans the whole run — including the post-queryable
    // enhancing tail — and stops when the user finishes (dialog leaves the
    // index step). Keeping a fixed interval avoids stopping early during a brief
    // window where the status plane momentarily reports nothing active (e.g.
    // between "queryable" and the enhancing flag flipping on).
    refetchInterval: () => POLL_MS,
    // A missing status sidecar is a normal "not indexed yet" case, not an error
    // worth retrying aggressively.
    retry: false,
  });
}
