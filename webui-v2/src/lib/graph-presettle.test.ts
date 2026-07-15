import { describe, it, expect, vi } from "vitest";
import {
  presettleToConvergence,
  DEFAULT_MAX_PRESETTLE_STEPS,
  type PresettleEngine,
} from "./graph-presettle";

/**
 * Fake engine: `progress` climbs by `perStep` on every step() until it
 * reaches 1. Records the call order so we can assert start→pause→step.
 */
function makeFakeEngine(perStep: number) {
  const calls: string[] = [];
  let progress = 0;
  const engine: PresettleEngine = {
    start: vi.fn(() => calls.push("start")),
    pause: vi.fn(() => calls.push("pause")),
    step: vi.fn(() => {
      calls.push("step");
      progress = Math.min(1, progress + perStep);
    }),
    get progress() {
      return progress;
    },
  };
  return { engine, calls };
}

describe("presettleToConvergence", () => {
  it("injects energy then pauses BEFORE stepping (no painted ticks)", () => {
    const { engine, calls } = makeFakeEngine(0.25);
    presettleToConvergence(engine);
    expect(engine.start).toHaveBeenCalledWith(1);
    expect(calls[0]).toBe("start");
    expect(calls[1]).toBe("pause");
    expect(calls[2]).toBe("step");
  });

  it("stops as soon as the sim converges (progress >= 1)", () => {
    const { engine } = makeFakeEngine(0.25); // converges in 4 steps
    const steps = presettleToConvergence(engine);
    expect(steps).toBe(4);
    expect(engine.step).toHaveBeenCalledTimes(4);
  });

  it("never exceeds the max-steps guard for a non-converging sim", () => {
    const { engine } = makeFakeEngine(0); // progress never advances
    const steps = presettleToConvergence(engine, 50);
    expect(steps).toBe(50);
    expect(engine.step).toHaveBeenCalledTimes(50);
  });

  it("defaults the guard to DEFAULT_MAX_PRESETTLE_STEPS", () => {
    const { engine } = makeFakeEngine(0);
    const steps = presettleToConvergence(engine);
    expect(steps).toBe(DEFAULT_MAX_PRESETTLE_STEPS);
  });

  it("does not step at all when the engine is already settled", () => {
    const { engine } = makeFakeEngine(1);
    // Force progress to 1 up-front by stepping once via a pre-run… instead,
    // simulate an already-converged engine:
    const settled: PresettleEngine = {
      start: vi.fn(),
      pause: vi.fn(),
      step: vi.fn(),
      get progress() {
        return 1;
      },
    };
    const steps = presettleToConvergence(settled);
    expect(steps).toBe(0);
    expect(settled.step).not.toHaveBeenCalled();
    // engine unused in this case
    void engine;
  });
});
