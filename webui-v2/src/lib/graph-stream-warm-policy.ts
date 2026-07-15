/* ============================================================
   lib/graph-stream-warm-policy.ts — pure warm retry/give-up policy +
   backend SSE error-event parsing for the progressive graph stream (#5722).

   Extracted as a pure seam (mirrors graph-stream-reducer) so the two
   decisions the SSE consumer (hooks/use-graph-stream) has to make on a
   pre-meta failure are unit-testable without a live EventSource:

     • decideWarmRetry — given how many warm attempts have already been
       made, either retry with the next backoff delay or GIVE UP with a
       user-facing message once the ceiling is reached. Without a ceiling a
       group whose warm attempt keeps failing (not just "still warming")
       spins the "Warming index…" affordance forever with no way out (#5722).
     • parseWarmErrorEvent — parses the backend's distinguishable `error` SSE
       event (event: error, data: {"code","message"}) into a safe shape, with
       a generic fallback if the payload is malformed.
   ============================================================ */

// Reconnect/retry backoff for a COLD group (503 → warming, or a genuine load
// failure surfaced via the `error` event). Bounded + capped so a slow warm
// self-heals without hammering the endpoint.
export const WARM_BACKOFF_MS = [1000, 2000, 3500, 5000];

// #5722 — retry ceiling. Once this many warm attempts have been made with no
// success, give up and surface an error instead of retrying forever. Chosen
// generously (comfortably longer than any real warm, even a slow one) so a
// genuinely slow-but-eventually-successful warm is never cut short, while a
// warm that is truly stuck or failing repeatedly still resolves to a visible
// error within a bounded, human-scale window.
export const MAX_WARM_ATTEMPTS = 8;

export type WarmDecision =
  | { kind: "retry"; delayMs: number }
  | { kind: "giveUp"; message: string };

/**
 * Decide what to do after the `attempt`-th warm failure (0-indexed: the
 * FIRST failure passes attempt=0). Retries with the capped backoff schedule
 * until MAX_WARM_ATTEMPTS is reached, then gives up with a helpful message.
 */
export function decideWarmRetry(attempt: number): WarmDecision {
  if (attempt >= MAX_WARM_ATTEMPTS) {
    return {
      kind: "giveUp",
      message:
        "The graph is taking too long to load. It may have failed to warm up — try again, or check the daemon.",
    };
  }
  const idx = Math.min(attempt, WARM_BACKOFF_MS.length - 1);
  return { kind: "retry", delayMs: WARM_BACKOFF_MS[idx] };
}

/**
 * Reset the warm-retry budget after a server `warming` heartbeat (#48).
 *
 * The backend now keeps the SSE connection open on a cold group and flushes
 * `warming` heartbeats while it performs a BOUNDED blocking warm, instead of
 * returning a bare 503. A heartbeat is server-confirmed progress, so a genuinely
 * large graph that legitimately takes a long time to warm must not be cut off by
 * the give-up ceiling: every heartbeat resets the attempt counter to 0, so the
 * client keeps waiting/retrying as long as the server is demonstrably still
 * working. The ceiling then only fires when heartbeats STOP (a truly stuck or
 * failed warm), never on a slow-but-progressing one.
 */
export function warmAttemptAfterHeartbeat(_current: number): number {
  return 0;
}

/**
 * Backend `error` SSE codes that mean "still warming, just slow" rather than a
 * terminal failure (#50). On these the consumer must RECONNECT and keep waiting
 * (bounded by the retry ceiling) instead of surfacing `error` and falling back
 * to the uncapped, blocking full-payload blob — which re-pays the SAME slow load
 * and hangs. `warm_timeout` is the bounded-warm deadline elapsing; the graph is
 * still loading server-side, so a reconnect resumes the warm.
 */
const RECONNECTABLE_WARM_CODES = new Set<string>(["warm_timeout"]);

/**
 * Whether a backend `error` SSE event with this code should trigger a reconnect
 * (keep waiting) rather than a fall-back-to-blob error (#50).
 */
export function isReconnectableWarmError(code: string): boolean {
  return RECONNECTABLE_WARM_CODES.has(code);
}

/** Parsed shape of the backend's `error` SSE event (v2GraphStreamError). */
export interface WarmErrorDetail {
  code: string;
  message: string;
}

const GENERIC_WARM_ERROR_MESSAGE = "Could not load the graph.";

/**
 * Parse the backend's `error` SSE event payload (event: error, data:
 * {"code","message"}). Falls back to a generic code/message when the payload
 * is malformed or missing fields, so a parse failure never leaves the caller
 * without a renderable error state.
 */
export function parseWarmErrorEvent(raw: string): WarmErrorDetail {
  try {
    const parsed = JSON.parse(raw) as { code?: unknown; message?: unknown };
    const code = typeof parsed.code === "string" && parsed.code ? parsed.code : "unknown";
    const message =
      typeof parsed.message === "string" && parsed.message
        ? parsed.message
        : GENERIC_WARM_ERROR_MESSAGE;
    return { code, message };
  } catch {
    return { code: "unknown", message: GENERIC_WARM_ERROR_MESSAGE };
  }
}
