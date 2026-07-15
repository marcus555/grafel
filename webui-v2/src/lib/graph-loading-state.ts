/* ============================================================
   lib/graph-loading-state.ts — pure loading-state derivation for the Graph
   screen (#48).

   The screen has two data sources: the progressive SSE stream (primary) and,
   only if the stream genuinely fails, the full-payload blob fetch (fallback).
   The hard requirement is that the screen NEVER sits at a blank "loading"
   forever (the old TTFB=0 hang): `isLoading` must clear the instant ANYTHING
   is renderable — the first streamed nodes, or the fallback blob's first data
   — and a cold group must show a "warming" affordance rather than a dead
   blank.

   Extracted as a pure seam (mirrors graph-stream-reducer / warm-policy) so the
   derivation is unit-testable without mounting the React tree.
   ============================================================ */

export type GraphStreamPhaseLike =
  | "idle"
  | "warming"
  | "streaming"
  | "done"
  | "error";

export interface GraphLoadingInput {
  /** Current SSE stream phase. */
  streamPhase: GraphStreamPhaseLike;
  /** True once the stream's `meta` event has been applied. */
  streamHasMeta: boolean;
  /** Nodes accumulated from the stream so far. */
  streamNodeCount: number;
  /** True when the stream has failed and the blob fallback is in charge. */
  fallbackActive: boolean;
  /** react-query isLoading for the fallback blob fetch. */
  fallbackIsLoading: boolean;
  /** Nodes available from the fallback blob (0 until it resolves). */
  fallbackNodeCount: number;
}

export interface GraphLoadingState {
  /** True ONLY while nothing is renderable yet. */
  isLoading: boolean;
  /** Show the "warming index…" affordance (stream-only, pre-meta cold group). */
  isWarming: boolean;
  /** Show the "building graph… N / total" progress affordance. */
  showProgress: boolean;
}

/**
 * Derive the Graph screen's loading/progress state from the two sources.
 *
 * Key invariant (#48): isLoading is driven off "first data received", not
 * "whole payload settled". For the fallback that means it clears as soon as
 * the blob yields ANY node, even if react-query still reports isLoading; for
 * the stream it clears on the first streamed nodes after meta.
 */
export function deriveGraphLoading(input: GraphLoadingInput): GraphLoadingState {
  if (input.fallbackActive) {
    // Fallback (blob) in charge: renderable the moment it has any node.
    const hasData = input.fallbackNodeCount > 0;
    return {
      isLoading: input.fallbackIsLoading && !hasData,
      isWarming: false,
      showProgress: false,
    };
  }

  // Stream in charge: renderable once meta + at least one node have landed.
  const hasRenderable = input.streamHasMeta && input.streamNodeCount > 0;
  const isWarming = input.streamPhase === "warming";
  const showProgress =
    input.streamPhase === "warming" ||
    input.streamPhase === "streaming" ||
    (input.streamPhase === "done" && input.streamNodeCount === 0);
  return {
    isLoading: !hasRenderable,
    isWarming,
    showProgress,
  };
}
