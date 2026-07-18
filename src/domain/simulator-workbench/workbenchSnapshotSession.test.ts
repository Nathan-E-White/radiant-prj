import { describe, expect, it, vi } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { WorkbenchReadError, type AcceptedWorkbenchSnapshot, type WorkbenchSnapshotAdapter } from "./liveWorkbench";
import {
  createWorkbenchSnapshotSession,
  type WorkbenchSnapshotSessionResult
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
    session.subscribe(({ readState }) => observations.push(`${readState.phase}:${readState.message}`));

    await session.refresh();
    expect(adapter.load).not.toHaveBeenCalled();

    await session.start();
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 1 } });
    await session.start();
    expect(adapter.load).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(500);
    expect(adapter.load).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(500);
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 2 } });

    await vi.advanceTimersByTimeAsync(500);
    await session.refresh();
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 3 } });
    await vi.advanceTimersByTimeAsync(500);
    expect(adapter.load).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(500);
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 4 } });
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
    expect(session.getResult().readState).toMatchObject({
      phase: "fixture",
      errorKind: "unavailable",
      message: "offline Using the explicit local-demo fixture Snapshot.",
      model: {
        source: "fixture",
        healthPanelModel: {
          lifecycle: { summary: "2/2 complete" },
          worker: { summary: "2/2 nominal" }
        }
      }
    });

    await session.refresh();
    expect(session.getResult().readState).toMatchObject({
      phase: "fixture",
      errorKind: "unavailable",
      message: "offline still Retaining the explicit whole-Snapshot fixture fallback.",
      model: { source: "fixture" }
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

  it("retains accepted live data as stale across partial failure and generation regression", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const adapter = sequenceAdapter([
      accepted(8, fixtureInput),
      new WorkbenchReadError("partial", "truncated"),
      accepted(7, fixtureInput),
      accepted(8, fixtureInput),
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
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 8 } });

    await session.refresh();
    expect(session.getResult().readState).toMatchObject({ phase: "live", model: { generation: 9 } });
    session.dispose();
  });

  it("publishes projection and Simulation Health from one accepted Snapshot generation", async () => {
    const fixtureInput = loadFixtureWorkbenchData();
    const generationEight = withHealthState(fixtureInput, {
      generatedAt: "2026-07-18T11:59:55Z",
      activeSimulationRuns: [{
        runId: "distinctive-live-run",
        scenarioId: "live-scheduler-drift",
        lifecycle: "streaming",
        health: "nominal",
        artifactStatus: "committed",
        valueBasis: "simulated"
      }]
    });
    const generationNine = withHealthState(fixtureInput, {
      generatedAt: "2026-07-18T12:00:25Z",
      activeSimulationRuns: [{
        runId: "recovered-run-a",
        scenarioId: "recovered-scenario",
        lifecycle: "completed",
        health: "nominal",
        artifactStatus: "staged",
        valueBasis: "simulated"
      }, {
        runId: "recovered-run-b",
        scenarioId: "recovered-scenario",
        lifecycle: "completed",
        health: "nominal",
        artifactStatus: "staged",
        valueBasis: "simulated"
      }]
    });
    const observedTimes = [
      new Date("2026-07-18T12:00:00Z"),
      new Date("2026-07-18T12:00:30Z")
    ];
    const session = createWorkbenchSnapshotSession(sequenceAdapter([
      accepted(8, generationEight),
      new WorkbenchReadError("unavailable", "offline"),
      accepted(9, generationNine)
    ]), {
      allowFixtureFallback: true,
      fixtureInput,
      refreshIntervalMs: 60_000,
      now: () => observedTimes.shift() ?? new Date("2026-07-18T12:00:30Z")
    });

    await session.start();
    const acceptedGenerationEight = session.getResult().readState.model;
    expect(acceptedGenerationEight).toMatchObject({
      generation: 8,
      source: "live",
      healthPanelModel: {
        lifecycle: { summary: "0/1 complete", status: "degraded" },
        worker: { summary: "1/1 nominal", status: "healthy" },
        artifact: { detail: "committed", status: "healthy" },
        streamFreshness: { summary: "fresh", status: "healthy" }
      }
    });
    expect(session.getResult().readState.message).toBe("Live Workbench generation 8 accepted atomically.");

    await session.refresh();
    expect(session.getResult().readState.model).toBe(acceptedGenerationEight);
    expect(session.getResult().readState).toMatchObject({ phase: "stale", model: { generation: 8 } });

    await session.refresh();
    expect(session.getResult().readState.model).not.toBe(acceptedGenerationEight);
    expect(session.getResult().readState).toMatchObject({
      phase: "live",
      model: {
        generation: 9,
        healthPanelModel: {
          lifecycle: { summary: "2/2 complete", status: "healthy" },
          worker: { summary: "2/2 nominal", status: "healthy" },
          artifact: { detail: "staged", status: "healthy" },
          streamFreshness: { summary: "fresh", status: "healthy" }
        }
      }
    });
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
