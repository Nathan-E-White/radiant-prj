import { describe, expect, it, vi } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { WorkbenchReadError, type AcceptedWorkbenchSnapshot, type WorkbenchSnapshotAdapter } from "./liveWorkbench";
import {
  createBrowserWorkbenchSnapshotSession,
  createWorkbenchSnapshotSession,
  type WorkbenchSnapshotScheduler,
  type WorkbenchSnapshotSessionOptions
} from "./workbenchSnapshotSession";

describe("Workbench Snapshot session", () => {
  it("constructs the browser session with an immutable initial projected result", () => {
    const session = createBrowserWorkbenchSnapshotSession({ selectedUnitId: "KAL-03" });
    expect(session.getState()).toMatchObject({
      readState: { phase: "loading", model: null },
      projection: null,
      selection: { selectedUnitId: "KAL-03" }
    });
    session.dispose();
  });

  it("uses browser timer defaults only after a successful start and cancels them on disposal", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const schedule = vi.spyOn(globalThis, "setTimeout");
    const cancel = vi.spyOn(globalThis, "clearTimeout");
    try {
      const inactive = createWorkbenchSnapshotSession(sequenceAdapter([accepted(1, fixtureInput)]), {
        allowFixtureFallback: false,
        fixtureInput,
        refreshIntervalMs: 10_000
      });
      inactive.dispose();
      expect(cancel).not.toHaveBeenCalled();

      const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(1, fixtureInput)]), {
        allowFixtureFallback: false,
        fixtureInput,
        refreshIntervalMs: 10_000
      });
      await session.start();
      expect(schedule).toHaveBeenCalledWith(expect.any(Function), 10_000);
      const handle = schedule.mock.results.at(-1)?.value;
      session.dispose();
      expect(cancel).toHaveBeenCalledWith(handle);
    } finally {
      schedule.mockRestore();
      cancel.mockRestore();
    }
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
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 1 } });
    await session.start();
    expect(adapter.load).toHaveBeenCalledTimes(1);

    await scheduler.advanceBy(500);
    expect(adapter.load).toHaveBeenCalledTimes(1);
    await scheduler.advanceBy(500);
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 2 } });

    await scheduler.advanceBy(500);
    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 3 } });
    await scheduler.advanceBy(500);
    expect(adapter.load).toHaveBeenCalledTimes(3);
    await scheduler.advanceBy(500);
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 4 } });
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
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 2 } });
    session.dispose();
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
    expect(session.getState().readState).toMatchObject({
      phase: "fixture",
      errorKind: "unavailable",
      model: { source: "fixture", acceptedAt: "2026-07-18T12:00:00.000Z" }
    });

    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "error", errorKind: "auth", model: { source: "fixture" } });

    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 4, source: "live" } });
    session.dispose();

    const emptySession = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("empty", "empty store")]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await emptySession.start();
    expect(emptySession.getState().readState).toMatchObject({ phase: "fixture", errorKind: "empty", model: { source: "fixture" } });
    emptySession.dispose();

    const rejectedSession = createWorkbenchSnapshotSession(
      sequenceAdapter([
        new WorkbenchReadError("auth", "denied"),
        new WorkbenchReadError("unavailable", "offline later")
      ]),
      { allowFixtureFallback: true, fixtureInput, refreshIntervalMs: 60_000 }
    );
    await rejectedSession.start();
    expect(rejectedSession.getState().readState).toMatchObject({ phase: "error", errorKind: "auth", model: null });
    await rejectedSession.refresh();
    expect(rejectedSession.getState().readState).toMatchObject({ phase: "error", errorKind: "unavailable", model: null });
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
      expect(session.getState().readState).toMatchObject({ phase: "error", errorKind: kind, model: null });
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
    expect(disabled.getState().readState).toMatchObject({ phase: "error", errorKind: "unavailable", model: null });
    disabled.dispose();

    options.allowFixtureFallback = true;
    const enabled = createWorkbenchSnapshotSession(
      sequenceAdapter([new WorkbenchReadError("empty", "empty")]),
      options
    );
    options.allowFixtureFallback = false;
    fixtureInput.state.scenarioId = "mutated-after-creation";
    await enabled.start();
    expect(enabled.getState().readState).toMatchObject({
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
    const acceptedModel = session.getState().readState.model;
    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "stale", errorKind: "partial", model: { generation: 8 } });
    expect(session.getState().readState.model).toBe(acceptedModel);

    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "stale", errorKind: "generation", model: { generation: 8 } });

    await session.refresh();
    expect(session.getState().readState).toMatchObject({
      phase: "live",
      model: { generation: 8, input: { state: { scenarioId: "equal-generation-refresh" } } }
    });

    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 9 } });
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
    expect(session.getState().readState.phase).toBe("loading");
    pending[1]?.resolve(accepted(12, fixtureInput));
    await second;
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 12 } });

    const third = session.refresh();
    session.dispose();
    expect(pending[2]?.signal.aborted).toBe(true);
    await third;
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 12 } });
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
    const exposed = session.getState().readState;
    expect(() => {
      if (!exposed.model) throw new Error("missing accepted model");
      exposed.model.generation = 2;
    }).toThrow(TypeError);
    expect(() => {
      if (!exposed.model) throw new Error("missing accepted model");
      exposed.model.input.state.schemaVersion = "corrupted" as typeof exposed.model.input.state.schemaVersion;
    }).toThrow(TypeError);
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 8 } });
    expect(Object.isFrozen(fixtureInput)).toBe(false);

    await session.refresh();
    expect(session.getState().readState).toMatchObject({ phase: "live", model: { generation: 9 } });
    session.dispose();
  });

  it("publishes read state, effective selection, and projection as one immutable result", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(5, fixtureInput)]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000,
      initialSelection: { selectedUnitId: "KAL-03", selectedValueId: "missing-value" }
    });

    expect(session.getState()).toMatchObject({ readState: { phase: "loading" }, projection: null });
    await session.start();
    const initial = session.getState();
    expect(initial.selection.selectedUnitId).toBe("KAL-03");
    expect(initial.selection.selectedValueId).toBe(initial.projection?.selectedValue?.valueId);
    expect(initial.projection?.selectedValue?.valueBasis).toBe("imputed");
    expect(Object.isFrozen(initial)).toBe(true);
    expect(Object.isFrozen(initial.projection)).toBe(true);
    expect(Object.isFrozen(initial.selection)).toBe(true);

    for (const basis of ["measured", "imputed", "simulated"] as const) {
      const value = session.getState().projection?.groups[basis].values[0];
      if (!value) throw new Error(`missing visible ${basis} value`);
      session.selectValue(value.valueId);
      const selected = session.getState();
      expect(selected.selection.selectedValueId).toBe(value.valueId);
      expect(selected.projection?.selectedValue?.valueBasis).toBe(basis);
      expect(selected.projection?.viewport.layers.find((layer) => layer.valueId === value.valueId)?.selected).toBe(true);
      expect(selected.projection?.explanation.basisLabel).toBe(basis);
    }

    const missingLineage = session.getState().projection?.groups.imputed.values.find((value) => !value.lineage);
    if (!missingLineage) throw new Error("missing explicit missing-Lineage fixture value");
    session.selectValue(missingLineage.valueId);
    expect(session.getState().projection).toMatchObject({
      selectedValue: { valueId: missingLineage.valueId },
      selectedLineage: null,
      selectedLineageMissing: true,
      explanation: { kind: "engineering" }
    });

    const otherUnitValue = fixtureInput.twin.entities
      .find((entity) => entity.unitId !== "KAL-03")
      ?.values[0]?.valueId;
    if (!otherUnitValue) throw new Error("missing other-unit fixture value");
    const beforeInvalidValue = session.getState();
    session.selectValue(otherUnitValue);
    session.selectValue("missing-value");
    expect(session.getState()).toBe(beforeInvalidValue);

    const beforeInvalidUnit = session.getState();
    session.selectUnit("missing-unit");
    expect(session.getState()).toBe(beforeInvalidUnit);

    const oldUnitValueId = initial.selection.selectedValueId;
    session.selectUnit("KAL-02");
    const commercialUnit = session.getState();
    expect(commercialUnit.selection).toMatchObject({
      selectedUnitId: "KAL-02",
      selectedCommercialBasisId: "CDB-KAL-02-FACILITY-HEAT"
    });
    expect(commercialUnit.selection.selectedValueId).not.toBe(oldUnitValueId);
    expect(commercialUnit.selection.selectedValueId).toBe(commercialUnit.projection?.selectedValue?.valueId);
    expect(commercialUnit.projection?.explanation.kind).toBe("commercial");
    expect(commercialUnit.projection?.fleetUnits.find((unit) => unit.unitId === "KAL-02")?.selected).toBe(true);

    const repeatedListener = vi.fn();
    const unsubscribeRepeated = session.subscribe(repeatedListener);
    session.selectUnit("KAL-02");
    expect(session.getState()).toBe(commercialUnit);
    expect(repeatedListener).toHaveBeenCalledTimes(1);
    unsubscribeRepeated();

    const beforeUnknownValue = session.getState();
    session.selectValue("missing-value");
    expect(session.getState()).toBe(beforeUnknownValue);
    session.dispose();
  });

  it("uses an engineering explanation when the selected unit has no owned commercial basis", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    fixtureInput.commercialDisplayBasis = fixtureInput.commercialDisplayBasis.filter((basis) => basis.unitId !== "KAL-02");
    const session = createWorkbenchSnapshotSession(sequenceAdapter([accepted(3, fixtureInput)]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    session.selectUnit("KAL-02");
    expect(session.getState()).toMatchObject({
      selection: { selectedUnitId: "KAL-02" },
      projection: { explanation: { kind: "engineering" } }
    });
    expect(session.getState().selection.selectedCommercialBasisId).toBeUndefined();
    session.dispose();
  });

  it("does not renegotiate selection while retaining the same accepted Snapshot as stale", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const unit02 = fixtureInput.fleetUnits.find((unit) => unit.unitId === "KAL-02");
    if (!unit02) throw new Error("missing KAL-02 fixture");
    unit02.commercialBasisId = "CDB-KAL-02-NOT-PUBLISHED";
    const session = createWorkbenchSnapshotSession(sequenceAdapter([
      accepted(3, fixtureInput),
      new WorkbenchReadError("unavailable", "offline")
    ]), {
      allowFixtureFallback: false,
      fixtureInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    session.selectUnit("KAL-02");
    expect(session.getState().selection.selectedCommercialBasisId).toBeUndefined();

    await session.refresh();
    expect(session.getState()).toMatchObject({
      readState: { phase: "stale", model: { generation: 3 } },
      selection: { selectedUnitId: "KAL-02" },
      projection: { explanation: { kind: "engineering" } }
    });
    expect(session.getState().selection.selectedCommercialBasisId).toBeUndefined();
    session.dispose();
  });

  it("reconciles unit, commercial basis, value, and Lineage across atomic Snapshot replacement", async () => {
    const initialInput = loadFixtureWorkbenchData();
    const equalInput = structuredClone(initialInput);
    equalInput.state.scenarioId = "equal-replacement";
    const replacementBasisInput = structuredClone(equalInput);
    replacementBasisInput.state.scenarioId = "replacement-basis";
    const unit02 = replacementBasisInput.fleetUnits.find((unit) => unit.unitId === "KAL-02");
    const unit02Basis = replacementBasisInput.commercialDisplayBasis.find((basis) => basis.unitId === "KAL-02");
    if (!unit02 || !unit02Basis) throw new Error("missing KAL-02 replacement fixtures");
    unit02.commercialBasisId = "CDB-KAL-02-REPLACEMENT";
    unit02Basis.basisId = "CDB-KAL-02-REPLACEMENT";

    const missingUnitInput = structuredClone(replacementBasisInput);
    missingUnitInput.state.scenarioId = "missing-selected-unit";
    missingUnitInput.fleetUnits = missingUnitInput.fleetUnits.filter((unit) => unit.unitId !== "KAL-02");

    const emptyValueInput = structuredClone(missingUnitInput);
    emptyValueInput.state.scenarioId = "empty-selected-unit";
    emptyValueInput.twin.entities = emptyValueInput.twin.entities.filter((entity) => entity.unitId !== "KAL-01");
    emptyValueInput.commercialDisplayBasis = emptyValueInput.commercialDisplayBasis.filter(
      (basis) => basis.unitId !== "KAL-01"
    );

    const session = createWorkbenchSnapshotSession(sequenceAdapter([
      accepted(5, initialInput),
      accepted(5, equalInput),
      accepted(6, replacementBasisInput),
      accepted(7, missingUnitInput),
      accepted(8, emptyValueInput)
    ]), {
      allowFixtureFallback: false,
      fixtureInput: initialInput,
      refreshIntervalMs: 60_000
    });

    await session.start();
    session.selectUnit("KAL-02");
    const selectedValueId = session.getState().projection?.groups.measured.values[0]?.valueId;
    if (!selectedValueId) throw new Error("missing KAL-02 selected value fixture");
    session.selectValue(selectedValueId);

    await session.refresh();
    expect(session.getState()).toMatchObject({
      readState: { model: { generation: 5, input: { state: { scenarioId: "equal-replacement" } } } },
      selection: {
        selectedUnitId: "KAL-02",
        selectedValueId,
        selectedCommercialBasisId: "CDB-KAL-02-FACILITY-HEAT"
      },
      projection: { selectedValue: { valueId: selectedValueId } }
    });

    await session.refresh();
    expect(session.getState()).toMatchObject({
      readState: { model: { generation: 6, input: { state: { scenarioId: "replacement-basis" } } } },
      selection: { selectedUnitId: "KAL-02", selectedCommercialBasisId: "CDB-KAL-02-REPLACEMENT" },
      projection: { selectedValue: { valueId: selectedValueId }, explanation: { kind: "commercial" } }
    });

    await session.refresh();
    expect(session.getState()).toMatchObject({
      readState: { model: { generation: 7, input: { state: { scenarioId: "missing-selected-unit" } } } },
      selection: { selectedUnitId: "KAL-01", selectedCommercialBasisId: "CDB-KAL-01-PPA" },
      projection: { selectedUnit: { unitId: "KAL-01" }, selectedValue: { valueBasis: "imputed" } }
    });

    await session.refresh();
    const empty = session.getState();
    expect(empty.readState.model).toMatchObject({ generation: 8, input: { state: { scenarioId: "empty-selected-unit" } } });
    expect(empty.selection).toEqual({ selectedUnitId: "KAL-01" });
    expect(empty.projection).toMatchObject({
      selectedValue: null,
      selectedLineage: null,
      selectedLineageMissing: false,
      explanation: { kind: "engineering", subtitle: "No value selected" }
    });
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
