import { describe, expect, it, vi } from "vitest";
import { buildWorkbenchProjection } from "./projection";
import {
  createHttpWorkbenchDataAdapter,
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

    const invalidGeneration = liveSnapshot(7);
    invalidGeneration.generation = -1;
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidGeneration))).load()).rejects.toMatchObject({ kind: "schema" });

    const invalidState = liveSnapshot(7);
    invalidState.state.schemaVersion = "simulator-workbench.state.v2" as "simulator-workbench.state.v1";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidState))).load()).rejects.toMatchObject({ kind: "schema" });

    const invalidTwinBasis = liveSnapshot(7);
    invalidTwinBasis.twin.entities[0]!.values[0]!.valueBasis = "unknown" as "measured";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidTwinBasis))).load()).rejects.toMatchObject({ kind: "schema" });

    const invalidMeasured = liveSnapshot(7);
    invalidMeasured.measured[0]!.valueBasis = "simulated" as "measured";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidMeasured))).load()).rejects.toMatchObject({ kind: "schema" });

    const invalidResult = liveSnapshot(7);
    invalidResult.results[0]!.valueBasis = "measured" as "simulated";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidResult))).load()).rejects.toMatchObject({ kind: "schema" });

    const invalidLineage = liveSnapshot(7);
    invalidLineage.lineage[0]!.schemaVersion = "digital-twin.lineage.v2" as "digital-twin.lineage.v1";
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(invalidLineage))).load()).rejects.toMatchObject({ kind: "schema" });

    const empty = liveSnapshot(7);
    empty.measured = [];
    empty.results = [];
    empty.lineage = [];
    empty.twin.entities = [];
    await expect(createHttpWorkbenchDataAdapter(async () => new Response(JSON.stringify(empty))).load()).rejects.toMatchObject({ kind: "empty" });
  });

  it("classifies transport and HTTP failures for the session policy", async () => {
    await expect(createHttpWorkbenchDataAdapter(async () => Promise.reject(new Error("offline"))).load()).rejects.toMatchObject({
      kind: "unavailable",
      message: "offline"
    });

    for (const [status, kind] of [
      [401, "auth"],
      [403, "auth"],
      [204, "empty"],
      [404, "empty"],
      [502, "unavailable"],
      [503, "unavailable"],
      [504, "unavailable"],
      [500, "partial"]
    ] as const) {
      await expect(createHttpWorkbenchDataAdapter(async () => new Response(status === 204 ? null : "failure", { status })).load())
        .rejects.toMatchObject({ kind });
    }

    await expect(createHttpWorkbenchDataAdapter(async () => new Response("{")).load()).rejects.toMatchObject({ kind: "partial" });
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
