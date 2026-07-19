import { describe, expect, it, vi } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { WorkbenchReadError, type AcceptedWorkbenchSnapshot, type WorkbenchSnapshotAdapter } from "./liveWorkbench";
import {
  createWorkbenchSnapshotSession,
  type WorkbenchSnapshotScheduler,
  type WorkbenchSnapshotSessionOptions
} from "./workbenchSnapshotSession";

describe("Workbench Snapshot session", () => {
  it("exposes render-ready projection and selection commands in its result", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(1, fixtureInput)]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    expect(session.getResult()).toMatchObject({
      readState: { phase: "live", model: { generation: 1 } },
      selection: {
        selectedUnitId: "KAL-01",
        selectedValueId: "VAL-KAL-01-IMPUTED-CORE-DISTRIBUTION"
      },
      projection: {
        selectedUnit: { unitId: "KAL-01" },
        selectedValue: { valueId: "VAL-KAL-01-IMPUTED-CORE-DISTRIBUTION" }
      }
    });
    expect(session.getResult().selection).not.toHaveProperty("selectedCommercialBasisId");

    session.getResult().selectUnit("KAL-03", "CDB-KAL-03-DESALINATION");
    expect(session.getResult()).toMatchObject({
      selection: {
        selectedUnitId: "KAL-03",
        selectedValueId: expect.stringContaining("KAL-03"),
        selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
      },
      projection: {
        selectedUnit: { unitId: "KAL-03" },
        explanation: { kind: "commercial" }
      }
    });

    session.getResult().selectValue("VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT");
    expect(session.getResult().projection?.selectedValue?.valueId).toBe("VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT");

    session.getResult().selectCommercialBasis(undefined);
    expect(session.getResult().projection?.explanation.kind).toBe("engineering");
    expect(session.getResult().selection).toEqual({
      selectedUnitId: "KAL-03",
      selectedValueId: "VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT"
    });

    session.getResult().selectCommercialBasis("CDB-KAL-01-PPA");
    expect(session.getResult().selection).not.toHaveProperty("selectedCommercialBasisId");
    expect(session.getResult().projection?.explanation.kind).toBe("engineering");

    session.getResult().selectCommercialBasis("CDB-KAL-03-DESALINATION");
    expect(session.getResult().selection.selectedCommercialBasisId).toBe("CDB-KAL-03-DESALINATION");
    expect(session.getResult().projection?.explanation.kind).toBe("commercial");
    session.dispose();
  });

  it("accepts selection commands before the first Snapshot and reconciles them on acceptance", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(1, fixtureInput)]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    session.getResult().selectUnit("KAL-03", "CDB-KAL-03-DESALINATION");
    expect(session.getResult()).toMatchObject({
      readState: { phase: "loading", model: null },
      projection: null,
      selection: {
        selectedUnitId: "KAL-03",
        selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
      }
    });

    session.getResult().selectCommercialBasis(undefined);
    expect(session.getResult().selection).toEqual({ selectedUnitId: "KAL-03" });
    expect(session.getResult().selection).not.toHaveProperty("selectedCommercialBasisId");
    session.getResult().selectUnit("KAL-03", "CDB-KAL-03-DESALINATION");

    await session.start();
    expect(session.getResult()).toMatchObject({
      readState: { phase: "live", model: { generation: 1 } },
      projection: { selectedUnit: { unitId: "KAL-03" } },
      selection: {
        selectedUnitId: "KAL-03",
        selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
      }
    });
    session.dispose();
  });

  it("accepts a Snapshot with no selectable value without inventing a selection", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(1, withoutValues(fixtureInput))]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    expect(session.getResult()).toMatchObject({
      readState: { phase: "live", model: { generation: 1 } },
      projection: { selectedValue: null },
      selection: { selectedUnitId: "KAL-01" }
    });
    expect(session.getResult().selection).not.toHaveProperty("selectedValueId");
    session.dispose();
  });

  it("publishes one reconciled result when a replacement Snapshot changes available entities", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const withoutKal03 = withoutUnit(fixtureInput, "KAL-03");
    const session = createWorkbenchSnapshotSession(sequenceAdapter([
      accepted(8, fixtureInput),
      accepted(9, fixtureInput),
      accepted(10, withoutKal03)
    ]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });
    const observations: WorkbenchSnapshotSessionResult[] = [];
    session.subscribe((result) => observations.push(result));

    await session.start();
    session.getResult().selectUnit("KAL-03", "CDB-KAL-03-DESALINATION");
    session.getResult().selectValue("VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT");
    await session.refresh();
    expect(session.getResult()).toMatchObject({
      readState: { model: { generation: 9 } },
      selection: {
        selectedUnitId: "KAL-03",
        selectedValueId: "VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT",
        selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
      },
      projection: { selectedUnit: { unitId: "KAL-03" } }
    });

    await session.refresh();
    expect(session.getResult()).toMatchObject({
      readState: { model: { generation: 10 } },
      selection: { selectedUnitId: "KAL-01" },
      projection: { selectedUnit: { unitId: "KAL-01" } }
    });
    expect(session.getResult().selection).not.toHaveProperty("selectedCommercialBasisId");
    expect(observations.at(-1)).toBe(session.getResult());
    expect(observations.some((result) =>
      result.readState.model?.generation === 10 && result.projection?.selectedUnit.unitId === "KAL-03"
    )).toBe(false);
    session.dispose();
  });

  it("owns initial loading, scheduled refresh, and manual refresh", async () => {
    const scheduler = createTestScheduler();
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
      refreshIntervalMs: 1_000,
      scheduler
    });
    const observations: string[] = [];
    session.subscribe(({ readState }) => observations.push(`${readState.phase}:${readState.message}`));

    await session.refresh();
    expect(adapter.load).not.toHaveBeenCalled();

    await session.start();
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 1 } });
    await session.start();
    expect(adapter.load).toHaveBeenCalledTimes(1);

    await scheduler.advanceBy(500);
    expect(adapter.load).toHaveBeenCalledTimes(1);
    await scheduler.advanceBy(500);
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 2 } });

    await scheduler.advanceBy(500);
    await session.refresh();
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 3 } });
    await scheduler.advanceBy(500);
    expect(adapter.load).toHaveBeenCalledTimes(3);
    await scheduler.advanceBy(500);
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
    await scheduler.advanceBy(2_000);
    await session.refresh();
    expect(adapter.load).toHaveBeenCalledTimes(4);
  });

  it("lets a manual refresh supersede an older scheduled request", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const scheduler = createTestScheduler();
    const pending: Array<{ signal: AbortSignal; resolve: (value: AcceptedWorkbenchSnapshot) => void }> = [];
    let calls = 0;
    const adapter: WorkbenchSnapshotAdapter = {
      load: ({ signal } = {}) => {
        calls += 1;
        if (!signal) throw new Error("missing cancellation signal");
        if (calls === 1) return Promise.resolve(accepted(1, fixtureInput));
        return new Promise((resolve, reject) => {
          signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
          pending.push({ signal, resolve });
        });
      }
    };
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 1_000,
      scheduler
    });

    await session.start();
    await scheduler.advanceBy(1_000);
    expect(pending).toHaveLength(1);
    const manual = session.refresh();
    expect(pending[0]?.signal.aborted).toBe(true);
    expect(pending).toHaveLength(2);
    pending[1]?.resolve(accepted(2, fixtureInput));
    await manual;
    expect(session.getState()).toMatchObject({ phase: "live", model: { generation: 2 } });
    session.dispose();
  });

  it("uses fixtures only for an allowed initial unavailable or empty read and recovers live", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([
      new WorkbenchReadError("unavailable", "offline"),
      new WorkbenchReadError("unavailable", "offline still"),
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
    expect(session.getResult().readState).toMatchObject({ phase: "error", errorKind: "auth", model: { source: "fixture" } });

    await session.refresh();
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 4, source: "live" } });
    session.dispose();

    const emptySession = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("empty", "empty store")]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await emptySession.start();
    expect(emptySession.getResult().readState).toMatchObject({ phase: "fixture", errorKind: "empty", model: { source: "fixture" } });
    emptySession.dispose();

    const rejectedSession = createWorkbenchSnapshotSession(
      sequenceAdapter([
        new WorkbenchReadError("auth", "denied"),
        new WorkbenchReadError("unavailable", "offline later")
      ]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await rejectedSession.start();
    expect(rejectedSession.getResult().readState).toMatchObject({ phase: "error", errorKind: "auth", model: null });
    expect(rejectedSession.getResult().projection).toBeNull();
    await rejectedSession.refresh();
    expect(rejectedSession.getResult().readState).toMatchObject({ phase: "error", errorKind: "unavailable", model: null });
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
    const equalGenerationInput = structuredClone(fixtureInput);
    equalGenerationInput.state.scenarioId = "equal-generation-refresh";
    const adapter = sequenceAdapter([
      accepted(8, fixtureInput),
      new WorkbenchReadError("partial", "truncated"),
      accepted(7, fixtureInput),
      accepted(8, equalGenerationInput),
      accepted(9, fixtureInput)
    ]);
    const session = createWorkbenchSnapshotSession(adapter, {
      allowFixtureFallback: true,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    const acceptedModel = session.getResult().readState.model;
    await session.refresh();
    expect(session.getResult().readState).toMatchObject({ phase: "stale", errorKind: "partial", model: { generation: 8 } });
    expect(session.getResult().readState.model).toBe(acceptedModel);

    await session.refresh();
    expect(session.getResult().readState).toMatchObject({ phase: "stale", errorKind: "generation", model: { generation: 8 } });

    await session.refresh();
    expect(session.getState()).toMatchObject({
      phase: "live",
      model: { generation: 8, input: { state: { scenarioId: "equal-generation-refresh" } } }
    });

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
    expect(session.getResult().readState.phase).toBe("loading");
    pending[1]?.resolve(accepted(12, fixtureInput));
    await second;
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 12 } });

    const third = session.refresh();
    session.dispose();
    expect(pending[2]?.signal.aborted).toBe(true);
    await third;
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 12 } });
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

function withHealthState(
  input: ReturnType<typeof loadFixtureWorkbenchData>,
  state: Pick<ReturnType<typeof loadFixtureWorkbenchData>["state"], "generatedAt" | "activeSimulationRuns">
): ReturnType<typeof loadFixtureWorkbenchData> {
  return { ...input, state: { ...input.state, ...state } };
}

function withoutUnit(
  input: ReturnType<typeof loadFixtureWorkbenchData>,
  unitId: string
): ReturnType<typeof loadFixtureWorkbenchData> {
  return {
    ...input,
    state: {
      ...input.state,
      selectedUnitId: input.state.selectedUnitId === unitId ? undefined : input.state.selectedUnitId,
      fleetUnitRefs: input.state.fleetUnitRefs?.filter((ref) => ref !== unitId)
    },
    measured: input.measured.filter((frame) => frame.reactorId !== unitId),
    twin: {
      ...input.twin,
      entities: input.twin.entities.filter((entity) => entity.unitId !== unitId)
    },
    fleetUnits: input.fleetUnits.filter((unit) => unit.unitId !== unitId),
    commercialDisplayBasis: input.commercialDisplayBasis.filter((basis) => basis.unitId !== unitId)
  };
}

function withoutValues(
  input: ReturnType<typeof loadFixtureWorkbenchData>
): ReturnType<typeof loadFixtureWorkbenchData> {
  return {
    ...input,
    twin: {
      ...input.twin,
      entities: input.twin.entities.map((entity) => ({ ...entity, values: [] }))
    }
  };
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

function createTestScheduler(): WorkbenchSnapshotScheduler & { advanceBy(milliseconds: number): Promise<void> } {
  let now = 0;
  let nextHandle = 0;
  const tasks = new Map<number, { dueAt: number; task: () => void }>();
  return {
    schedule(task, delayMs) {
      const handle = ++nextHandle;
      tasks.set(handle, { dueAt: now + delayMs, task });
      return handle;
    },
    cancel(handle) {
      tasks.delete(handle as number);
    },
    async advanceBy(milliseconds) {
      now += milliseconds;
      for (const [handle, scheduled] of [...tasks].sort((left, right) => left[1].dueAt - right[1].dueAt)) {
        if (scheduled.dueAt > now) continue;
        tasks.delete(handle);
        scheduled.task();
        await Promise.resolve();
        await Promise.resolve();
      }
    }
  };
}
