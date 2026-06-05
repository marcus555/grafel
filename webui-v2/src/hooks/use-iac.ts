/* ============================================================
   hooks/use-iac.ts — IaC / Infrastructure data hook (#4256).

   Wraps api.getIaC in TanStack Query. Backed by the route
   GET /api/iac/{group} (handlers_iac.go → handleIaC), which
   returns a raw-JSON IaCReport: IaC resources grouped by tool
   (aws-cdk / pulumi / bicep / cloudformation / sam /
   serverless-framework / terraform), each with a cross-tool
   resource_category, tool-native type, the curated typed config
   properties stamped on the entity, and its relationship facets
   (IAM grants, event-source wiring, dependencies, stack/app/
   module topology). Pure static graph read.
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export const iacQueryKey = (groupId: string) => ["iac", groupId] as const;

/** IaC / Infrastructure report for the group. */
export function useIaC(groupId: string) {
  return useQuery({
    queryKey: iacQueryKey(groupId),
    queryFn: () => api.getIaC(groupId),
    enabled: !!groupId,
    staleTime: 30_000,
  });
}
