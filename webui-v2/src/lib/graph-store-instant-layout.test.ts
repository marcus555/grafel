/* Instant-layout default: a first-time user (nothing persisted) now gets
   instant layout ON; a user who explicitly toggled it OFF keeps that choice.
   The store reads localStorage at module init, so each case stubs a fresh
   localStorage and re-imports the store module. */
import { afterEach, describe, it, expect, vi } from "vitest";

const KEY = "ag.v2.graph.instantLayout";

function mockStorage(initial: Record<string, string> = {}): Storage {
  const store = new Map<string, string>(Object.entries(initial));
  return {
    get length() {
      return store.size;
    },
    getItem: (k: string) => (store.has(k) ? (store.get(k) as string) : null),
    setItem: (k: string, v: string) => {
      store.set(k, v);
    },
    removeItem: (k: string) => {
      store.delete(k);
    },
    clear: () => store.clear(),
    key: (i: number) => Array.from(store.keys())[i] ?? null,
  };
}

async function freshInstantLayout(): Promise<boolean> {
  vi.resetModules();
  const mod = await import("../store/use-graph-store");
  return mod.useGraphStore.getState().instantLayout;
}

describe("graph store — instantLayout default", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("defaults instantLayout ON when nothing is persisted", async () => {
    vi.stubGlobal("localStorage", mockStorage());
    expect(await freshInstantLayout()).toBe(true);
  });

  it("keeps an explicitly-persisted false (stored choice wins over the new default)", async () => {
    vi.stubGlobal("localStorage", mockStorage({ [KEY]: "false" }));
    expect(await freshInstantLayout()).toBe(false);
  });

  it("keeps an explicitly-persisted true", async () => {
    vi.stubGlobal("localStorage", mockStorage({ [KEY]: "true" }));
    expect(await freshInstantLayout()).toBe(true);
  });
});
