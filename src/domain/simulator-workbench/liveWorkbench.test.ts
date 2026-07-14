import { describe, expect, it, vi } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { buildWorkbenchProjection } from "./projection";
import {
  WorkbenchReadError,
  createHttpWorkbenchDataAdapter,
  createWorkbenchRefreshCoordinator,
  initialWorkbenchReadState,
  refreshWorkbenchReadState,
  type LiveWorkbenchSnapshot
} from "./liveWorkbench";

describe("live Workbench read boundary", () => {
  it("loads one coherent snapshot through one credential-free browser request", async () => {
    const fetcher = vi.fn(async () => new Response(JSON.stringify(liveSnapshot(4)), { status: 200 }));
    const accepted = await createHttpWorkbenchDataAdapter(fetcher).load();

    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledWith("/api/simulator-workbench/snapshot", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" }
    });
    expect(accepted.generation).toBe(4);
    expect(accepted.input.state.snapshotGeneration).toBe(4);
    expect(accepted.input.fleetUnits[0]).toMatchObject({
      unitId: "reactor-01",
      availabilityPhase: "status not provided",
      commercialMode: "commercial basis not provided",
      breakerToBreakerLabel: "output not provided"
    });
    expect(accepted.input.measured[0]?.valueBasis).toBe("measured");
    expect(accepted.input.twin.entities[0]?.values.map((value) => value.valueBasis)).toEqual([
      "measured",
      "imputed",
      "simulated",
      "simulated"
    ]);
    expect(accepted.input.lineages.map((lineage) => lineage.valueBasis)).toEqual(["imputed", "simulated"]);
    const projection = buildWorkbenchProjection(accepted.input);
    expect(projection.measuredFrames).toHaveLength(1);
    expect(projection.groups.measured.values).toHaveLength(1);
    expect(projection.groups.measured.values[0]).toMatchObject({
      valueId: "VAL-MEASURED-TAG-CORE",
      sourceQuality: "good"
    });
    expect(projection.groups.simulated.values.map((value) => value.valueId)).toContain("result-asset-only");
  });

  it("rejects schema and generation mismatches without assembling a partial projection", async () => {
    const mismatch = liveSnapshot(7);
    mismatch.state.snapshotGeneration = 6;
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(mismatch))).load()).rejects.toMatchObject({
      kind: "generation"
    });

    const invalid = liveSnapshot(7);
    invalid.twin.schemaVersion = "digital-twin.state.v2" as "digital-twin.state.v1";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalid))).load()).rejects.toMatchObject({
      kind: "schema"
    });

    const partial = liveSnapshot(7) as unknown as { twin: { entities: Array<{ entityId: string }> } };
    partial.twin.entities = [{ entityId: "reactor-01" }];
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(partial))).load()).rejects.toMatchObject({
      kind: "partial"
    });

    const noReactorIdentity = liveSnapshot(7);
    delete noReactorIdentity.measured[0]?.reactorId;
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(noReactorIdentity))).load()).rejects.toMatchObject({
      kind: "partial"
    });
  });

  it("uses fixtures only for an allowed initial unavailable or empty local-demo read", async () => {
    const unavailable = { load: async () => Promise.reject(new WorkbenchReadError("unavailable", "offline")) };
    const fixture = loadFixtureWorkbenchData();
    const state = await refreshWorkbenchReadState(initialWorkbenchReadState(), unavailable, {
      allowFixtureFallback: true,
      fixtureInput: fixture,
      now: () => new Date("2026-07-14T12:00:00Z")
    });
    expect(state.phase).toBe("fixture");
    expect(state.model?.source).toBe("fixture");
    expect(state.model?.input).toBe(fixture);

    const denied = await refreshWorkbenchReadState(initialWorkbenchReadState(), unavailable, {
      allowFixtureFallback: false,
      fixtureInput: fixture
    });
    expect(denied.phase).toBe("error");

    const auth = { load: async () => Promise.reject(new WorkbenchReadError("auth", "denied")) };
    const authState = await refreshWorkbenchReadState(initialWorkbenchReadState(), auth, {
      allowFixtureFallback: true,
      fixtureInput: fixture
    });
    expect(authState.phase).toBe("error");
    expect(authState.model).toBeNull();
  });

  it("retains the last coherent live snapshot as stale, then replaces it on recovery", async () => {
    const fixture = loadFixtureWorkbenchData();
    const live = createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(liveSnapshot(9))));
    const accepted = await refreshWorkbenchReadState(initialWorkbenchReadState(), live, {
      allowFixtureFallback: true,
      fixtureInput: fixture
    });
    expect(accepted.phase).toBe("live");

    const failed = await refreshWorkbenchReadState(
      accepted,
      { load: async () => Promise.reject(new WorkbenchReadError("partial", "truncated")) },
      { allowFixtureFallback: true, fixtureInput: fixture }
    );
    expect(failed.phase).toBe("stale");
    expect(failed.model).toBe(accepted.model);

    const recovered = await refreshWorkbenchReadState(
      failed,
      createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(liveSnapshot(10)))),
      { allowFixtureFallback: true, fixtureInput: fixture }
    );
    expect(recovered.phase).toBe("live");
    expect(recovered.model?.generation).toBe(10);
    expect(recovered.model).not.toBe(accepted.model);
  });

  it("keeps a newer live generation when a delayed older refresh arrives", async () => {
    const fixture = loadFixtureWorkbenchData();
    const accepted = await refreshWorkbenchReadState(
      initialWorkbenchReadState(),
      createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(liveSnapshot(12)))),
      { allowFixtureFallback: false, fixtureInput: fixture }
    );
    const older = await refreshWorkbenchReadState(
      accepted,
      createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(liveSnapshot(11)))),
      { allowFixtureFallback: false, fixtureInput: fixture }
    );
    expect(older.phase).toBe("stale");
    expect(older.errorKind).toBe("generation");
    expect(older.model?.generation).toBe(12);
  });

  it("aborts an overlapping refresh and the active refresh on disposal", async () => {
    const fixture = loadFixtureWorkbenchData();
    const pending: Array<{ signal: AbortSignal; resolve: (response: Response) => void }> = [];
    const fetcher = vi.fn((_url: string | URL | Request, init?: RequestInit) =>
      new Promise<Response>((resolve, reject) => {
        const signal = init?.signal;
        if (!signal) throw new Error("missing refresh signal");
        signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
        pending.push({ signal, resolve });
      })
    );
    const coordinator = createWorkbenchRefreshCoordinator(createHttpWorkbenchDataAdapter(fetcher), {
      allowFixtureFallback: false,
      fixtureInput: fixture
    });

    const first = coordinator.refresh(initialWorkbenchReadState());
    const second = coordinator.refresh(initialWorkbenchReadState());
    expect(pending[0]?.signal.aborted).toBe(true);
    pending[1]?.resolve(new Response(JSON.stringify(liveSnapshot(14))));
    await expect(first).resolves.toBeNull();
    await expect(second).resolves.toMatchObject({ phase: "live", model: { generation: 14 } });

    const third = coordinator.refresh(initialWorkbenchReadState());
    coordinator.dispose();
    expect(pending[2]?.signal.aborted).toBe(true);
    await expect(third).resolves.toBeNull();
  });
});

function liveSnapshot(generation: number): LiveWorkbenchSnapshot {
  return {
    generation,
    state: {
      schemaVersion: "simulator-workbench.state.v1",
      generatedAt: "2026-07-14T11:00:00Z",
      snapshotGeneration: generation,
      scenarioId: "scheduler-drift",
      valueBasisSummary: { measured: 1, imputed: 1, simulated: 1 },
      measuredStateRefs: ["scada_measured_frames"],
      twinStateRef: "digital_twin_state_values",
      lineageRefs: ["digital_twin_lineage"],
      activeSimulationRuns: [{ runId: "run-1", scenarioId: "scheduler-drift", lifecycle: "streaming", valueBasis: "simulated", health: "nominal", artifactStatus: "committed" }],
      panels: [
        { panelId: "measured", title: "Measured State", valueBasis: "measured" },
        { panelId: "simulated", title: "Simulated Result State", valueBasis: "simulated" }
      ]
    },
    measured: [{ schemaVersion: "scada.telemetry.v1", sourceId: "source-1", reactorId: "reactor-01", tagId: "TAG-CORE", assetId: "ASSET-CORE-A", signalKind: "flux", sampledAt: "2026-07-14T10:59:59Z", observedAt: "2026-07-14T11:00:00Z", sequence: 1, unit: "relative", value: { scalar: 0.81 }, quality: "good", valueBasis: "measured", syntheticStatus: "public-safe-standin" }],
    twin: {
      schemaVersion: "digital-twin.state.v1",
      twinId: "twin-live-1",
      asOf: "2026-07-14T11:00:00Z",
      entities: [{ entityId: "ASSET-CORE-A", displayName: "Core asset", values: [
        { valueId: "VAL-MEASURED-TAG-CORE", label: "Measured core signal", valueBasis: "measured", unit: "relative", value: { scalar: 0.81 }, confidence: 1, freshness: { ageSec: 1, status: "fresh" }, lineageId: "lin-measured", sourceIds: ["TAG-CORE"] },
        { valueId: "margin-imputed", label: "Core margin", valueBasis: "imputed", unit: "percent", value: { scalar: 14 }, confidence: 0.7, freshness: { ageSec: 4, status: "fresh" }, lineageId: "lin-imputed", sourceIds: ["TAG-CORE"] },
        { valueId: "margin-simulated", label: "Forecast margin", valueBasis: "simulated", unit: "percent", value: { scalar: 16 }, confidence: 0.6, freshness: { ageSec: 3, status: "fresh" }, lineageId: "lin-simulated", sourceIds: ["run-1"] }
      ] }]
    },
    lineage: [
      { schemaVersion: "digital-twin.lineage.v1", lineageId: "lin-imputed", valueId: "margin-imputed", valueBasis: "imputed", inputs: [{ sourceKind: "scada-tag", sourceId: "TAG-CORE", valueBasis: "measured" }], processingSteps: ["project"], artifacts: [] },
      { schemaVersion: "digital-twin.lineage.v1", lineageId: "lin-simulated", valueId: "margin-simulated", valueBasis: "simulated", inputs: [{ sourceKind: "simulation-run", sourceId: "run-1", valueBasis: "simulated" }], processingSteps: ["project"], artifacts: [] }
    ],
    results: [{ schemaVersion: "simops.result.v1", runId: "run-1", scenarioId: "scheduler-drift", workerId: "worker-1", workerKind: "scheduler", sequence: 1, producedAt: "2026-07-14T11:00:00Z", resultType: "syntheticEngineeringState", modelId: "model-1", inputWindow: { start: "2026-07-14T10:59:00Z", end: "2026-07-14T11:00:00Z" }, valueBasis: "simulated", syntheticStatus: "public-safe-standin", values: [{ resultId: "result-asset-only", entityId: "ASSET-CORE-A", valueId: "result-asset-only", label: "Asset-scoped simulated result", unit: "percent", value: { scalar: 18 }, confidence: 0.62 }] }]
  };
}
