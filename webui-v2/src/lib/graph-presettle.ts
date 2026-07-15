/* ============================================================
   lib/graph-presettle.ts — "Instant layout" pre-settle.

   The graph canvas normally ANIMATES the force layout: it seeds
   positions, `start(1)`s the cosmos.gl engine and lets the render
   loop paint every simulation tick as the nodes explode + settle
   (the "jiggle"). On large graphs that looks noisy.

   When the user opts into Instant layout we instead run the
   simulation to convergence SYNCHRONOUSLY — stepping the engine in
   a tight loop with the render loop paused, so no intermediate tick
   is ever painted — then pin the final positions in one frame.

   This module isolates the (pure, side-effect-only via the passed
   engine) pre-settle loop so it can be unit-tested against a fake
   engine without a GPU/WebGL context.
   ============================================================ */

/**
 * The minimal slice of the cosmos.gl `Graph` API the pre-settle loop drives.
 * Kept structural so tests can pass a lightweight fake.
 */
export interface PresettleEngine {
  /** Inject simulation energy (alpha). */
  start(alpha?: number): void;
  /** Halt the render-driven stepping so only our manual steps advance the sim. */
  pause(): void;
  /** Run one simulation step manually (works while paused). */
  step(): void;
  /** Convergence progress, 0 → 1. Reaches 1 when the layout has settled. */
  readonly progress: number;
}

/** Hard cap on manual steps so a non-converging sim can't spin forever. */
export const DEFAULT_MAX_PRESETTLE_STEPS = 600;

/**
 * Run `engine` to convergence WITHOUT painting intermediate ticks.
 *
 * Injects full energy, pauses the render loop, then steps the simulation
 * synchronously until it reports converged (`progress >= 1`) or `maxSteps`
 * is hit. Returns the number of manual steps taken (useful for tests /
 * telemetry). The caller is responsible for the subsequent fit + pin
 * (e.g. the canvas's `doSettle`).
 */
export function presettleToConvergence(
  engine: PresettleEngine,
  maxSteps: number = DEFAULT_MAX_PRESETTLE_STEPS,
): number {
  engine.start(1);
  engine.pause();
  let steps = 0;
  while (steps < maxSteps && engine.progress < 1) {
    engine.step();
    steps++;
  }
  return steps;
}
