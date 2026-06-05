/* ============================================================
   hooks/use-dataflow.ts — Data-flow & Taint data hook (#4265).

   Wraps api.getDataflow in TanStack Query. Backed by the route
   GET /api/dataflow/{group} (handlers_dataflow.go → handleDataflow),
   which returns a raw-JSON DataflowReport reading two sidecars:

     - taint sidecar → ranked SecurityFinding records (source→sink
       paths, confidence, vulnerability category), produced by
       internal/links/taint_flow.go;
     - data-flow sidecar → request-input → sink DATA_FLOWS_TO edges
       (tainted field, sink_kind, inter-procedural hop_path),
       produced by internal/links/dataflow_pass.go.

   Pure static graph + sidecar read. Honest-empty when neither
   sidecar exists for the group.
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export const dataflowQueryKey = (groupId: string) =>
  ["dataflow", groupId] as const;

/** Data-flow & taint report for the group. */
export function useDataflow(groupId: string) {
  return useQuery({
    queryKey: dataflowQueryKey(groupId),
    queryFn: () => api.getDataflow(groupId),
    enabled: !!groupId,
    staleTime: 30_000,
  });
}
