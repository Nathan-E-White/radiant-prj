import { describe, expect, it, vi } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { WorkbenchReadError, type AcceptedWorkbenchSnapshot, type WorkbenchSnapshotAdapter } from "./liveWorkbench";
import { createWorkbenchSnapshotSession, type WorkbenchSnapshotSessionOptions } from "./workbenchSnapshotSession";

describe("Workbench Snapshot session", () => {
  it("owns initial loading, scheduled refresh, and manual refresh", async () => {
    vi.useFakeTimers();
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([
      accepted(1, fixtureInput),
      accepted(2, fixtureInput),
      accepted(3, fixtureInput),
      accepted(4, fixtureInput)
    ]);
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 1_000
    });
    const observations: string[] = [];
    session.subscribe((state) => observations.push(`${state.phase}:${state.message}`));

    await session.refresh();
    expect(adapter.load).not.toHaveBeenCalled();

    await session.start();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 1 } });
    await session.start();
    expect(adapter.load).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(500);
    expect(adapter.load).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(500);
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 2 } });

    await vi.advanceTimersByTimeAsync(500);
    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 3 } });
    await vi.advanceTimersByTimeAsync(500);
    expect(adapter.load).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(500);
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 4 } });
    expect(observations.map((entry) => entry.split(":", 1)[0])).toEqual([
      "loading",
      "loading",
      "live",
      "recovering",
      "live",
      "recovering",
      "live",
      "recovering",
      "live"
    ]);
    expect(observations).toContain("recovering:Refreshing one coherent live Workbench Snapshot.");

    session.dispose();
    await vi.advanceTimersByTimeAsync(2_000);
    await session.refresh();
    expect(adapter.load).toHaveBeenCalledTimes(4);
    vi.useRealTimers();
  });

  it("uses fixtures only for an allowed initial unavailable or empty read and recovers live", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([
      new WorkbenchReadError("unavailable", "offline"),
      new WorkbenchReadError("auth", "denied"),
      accepted(4, fixtureInput)
    ]);
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: true,
      fixtureInput,
      refreshIntervalMs: 60_000,
      now: () => new Date("2026-07-18T12:00:00Z")
    });

    await session.start();
    expect(session.getState()).toMatchObject({
      phase: "fixture",
      errorKind: "unavailable",
      model: { source: "fixture", acceptedAt: "2026-07-18T12:00:00.000Z" }
    });

    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "error", errorKind: "auth", model: { source: "fixture" } });

    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 4, source: "live" } });
    session.dispose();

    const emptySession = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("empty", "empty store")]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await emptySession.start();
    expect(emptySession.getState()).toMatchObject({ phase: "fixture", errorKind: "empty", model: { source: "fixture" } });
    emptySession.dispose();

    const rejectedSession = createWorkbenchSnapshotSession(
      sequenceAdapter([
        new WorkbenchReadError("auth", "denied"),
        new WorkbenchReadError("unavailable", "offline later")
      ]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await rejectedSession.start();
    expect(rejectedSession.getState()).toMatchObject({ phase: "error", errorKind: "auth", model: null });
    await rejectedSession.refresh();
    expect(rejectedSession.getState()).toMatchObject({ phase: "error", errorKind: "unavailable", model: null });
    rejectedSession.dispose();
  });

  it.each(["auth", "schema", "generation", "partial"] as const)(
    "never substitutes fixtures for an initial %s failure",
    async (kind) => {
      const fixtureInput = loadFixtureWorkbenchData();
      const session = createWorkbenchSnapshotSession(
        sequenceAdapter([new WorkbenchReadError(kind, `${kind} failure`)]),
        { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
      );

      await session.start();
      expect(session.getState()).toMatchObject({ phase: "error", errorKind: kind, model: null });
      session.dispose();
    }
  );

  it("fixes disabled fixture configuration and fixture data when the session is created", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const originalScenario = fixtureInput.state.scenarioId;
    const options: WorkbenchSnapshotSessionOptions = {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    };
    const disabled = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("unavailable", "offline")]),
      options
    );
    options.allowFixtureFallback = true;
    await disabled.start();
    expect(disabled.getState()).toMatchObject({ phase: "error", errorKind: "unavailable", model: null });
    disabled.dispose();

    options.allowFixtureFallback = true;
    const enabled = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("empty", "empty")]),
      options
    );
    options.allowFixtureFallback = false;
    fixtureInput.state.scenarioId = "mutated-after-creation";
    await enabled.start();
    expect(enabled.getState()).toMatchObject({
      phase: "fixture",
      model: { source: "fixture", input: { state: { scenarioId: originalScenario } } }
    });
    enabled.dispose();
  });

  it("retains accepted live data as stale across partial failure and generation regression", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([
      accepted(8, fixtureInput),
      new WorkbenchReadError("partial", "truncated"),
      accepted(7, fixtureInput),
      accepted(9, fixtureInput)
    ]);
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: true,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    const acceptedModel = session.getState().model;
    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "stale", errorKind: "partial", model: { generation: 8 } });
    expect(session.getState().model).toBe(acceptedModel);

    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "stale", errorKind: "generation", model: { generation: 8 } });

    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 9 } });
    session.dispose();
  });

  it("cancels overlap and disposal without accepting superseded results", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const pending: Array<{
      signal: AbortSignal;
      resolve: (value: AcceptedWorkbenchSnapshot) => void;
    }> = [];
    const adapter: WorkbenchSnapshotAdapter = {
      load: ({ signal } = {}) => new Promise((resolve, reject) => {
        if (!signal) throw new Error("missing session cancellation signal");
        signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
        pending.push({ signal, resolve });
      })
    };
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    const first = session.start();
    const second = session.refresh();
    expect(pending[0]?.signal.aborted).toBe(true);
    await first;
    expect(session.getState().phase).toBe("loading");
    pending[1]?.resolve(accepted(12, fixtureInput));
    await second;
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 12 } });

    const third = session.refresh();
    session.dispose();
    expect(pending[2]?.signal.aborted).toBe(true);
    await third;
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 12 } });
  });

  it("stops publishing to unsubscribed listeners", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([accepted(2, fixtureInput)]);
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });
    const listener = vi.fn();
    const unsubscribe = session.subscribe(listener);
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();

    await session.start();
    expect(listener).toHaveBeenCalledTimes(1);
    session.dispose();
  });

  it("publishes owned immutable Snapshots that consumers cannot corrupt", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(8, fixtureInput), accepted(9, fixtureInput)]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    const exposed = session.getState();
    expect(() => {
      if (!exposed.model) throw new Error("missing accepted model");
      exposed.model.generation = 2;
    }).toThrow(TypeError);
    expect(() => {
      if (!exposed.model) throw new Error("missing accepted model");
      exposed.model.input.state.schemaVersion = "corrupted" as typeof exposed.model.input.state.schemaVersion;
    }).toThrow(TypeError);
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 8 } });
    expect(Object.isFrozen(fixtureInput)).toBe(false);

    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 9 } });
    session.dispose();
  });
});

function accepted(generation: number, input: ReturnType<typeof loadFixtureWorkbenchData>): AcceptedWorkbenchSnapshot {
  return { generation, source: "live", input };
}

function sequenceAdapter(
  outcomes: Array<AcceptedWorkbenchSnapshot | WorkbenchReadError>
): WorkbenchSnapshotAdapter & { load: ReturnType<typeof vi.fn> } {
  const load = vi.fn(async () => {
    const outcome = outcomes.shift();
    if (!outcome) throw new Error("unexpected Workbench Snapshot read");
    if (outcome instanceof Error) throw outcome;
    return outcome;
  });
  return { load };
}
