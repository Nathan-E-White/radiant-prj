import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getMeasuredState,
  getSimulatorWorkbenchState,
  getTwinState,
  getWorkbenchLineage,
  httpWorkbenchDataAdapter,
  type SimulatorWorkbenchState
} from "./simulatorWorkbench";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

describe("simulator workbench api client", () => {
  it("reads scaffolded workbench endpoints", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(sampleWorkbenchState()))
      .mockResolvedValueOnce(jsonResponse([]))
      .mockResolvedValueOnce(jsonResponse({ schemaVersion: "digital-twin.state.v1", twinId: "TWIN-001", asOf: "2026-07-06T15:00:00Z", entities: [] }))
      .mockResolvedValueOnce(jsonResponse({
        schemaVersion: "digital-twin.lineage.v1",
        lineageId: "LIN-001",
        valueId: "VAL-001",
        valueBasis: "imputed",
        inputs: [],
        processingSteps: ["scaffold"],
        artifacts: []
      }));
    globalThis.fetch = fetchMock;

    await expect(getSimulatorWorkbenchState()).resolves.toMatchObject({ scenarioId: "mixed-public-safe-twin-demo" });
    await expect(getMeasuredState()).resolves.toEqual([]);
    await expect(getTwinState()).resolves.toMatchObject({ twinId: "TWIN-001" });
    await expect(getWorkbenchLineage("VAL/001")).resolves.toMatchObject({ valueId: "VAL-001" });

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/simulator-workbench/state", expect.objectContaining({ method: "GET" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/simulator-workbench/measured", expect.objectContaining({ method: "GET" }));
    expect(fetchMock).toHaveBeenNthCalledWith(3, "/api/simulator-workbench/twin", expect.objectContaining({ method: "GET" }));
    expect(fetchMock).toHaveBeenNthCalledWith(4, "/api/simulator-workbench/lineage/VAL%2F001", expect.objectContaining({ method: "GET" }));
  });

  it("includes response bodies in thrown errors", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response("not wired yet", { status: 404 }));

    await expect(getSimulatorWorkbenchState()).rejects.toThrow("not wired yet");
  });

  it("keeps the HTTP data adapter parked for the presentational slice", async () => {
    globalThis.fetch = vi.fn();

    await expect(httpWorkbenchDataAdapter.load()).rejects.toThrow("parked");
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "Content-Type": "application/json" }
  });
}

function sampleWorkbenchState(): SimulatorWorkbenchState {
  return {
    schemaVersion: "simulator-workbench.state.v1",
    generatedAt: "2026-07-06T15:00:05Z",
    scenarioId: "mixed-public-safe-twin-demo",
    selectedUnitId: "KAL-01",
    valueBasisSummary: { measured: 1, imputed: 1, simulated: 1 },
    measuredStateRefs: ["examples/scada/telemetry.mixed.ndjson"],
    twinStateRef: "examples/digital-twin/twin-state.mixed.json",
    lineageRefs: ["examples/digital-twin/value-lineage.core-margin.json"],
    fleetUnitRefs: ["examples/simulator-workbench/fleet-units.mixed.json"],
    commercialDisplayBasisRefs: ["examples/simulator-workbench/commercial-display-basis.mixed.json"],
    activeSimulationRuns: [
      {
        runId: "RUN-SIMOPS-SCHED-DRIFT",
        scenarioId: "scheduler-drift",
        lifecycle: "complete",
        valueBasis: "simulated",
        health: "nominal",
        artifactStatus: "planned-reference"
      }
    ],
    panels: [
      { panelId: "measured-state", title: "Measured stand-ins", valueBasis: "measured" },
      { panelId: "digital-twin", title: "Imputed twin state", valueBasis: "imputed" },
      { panelId: "simulation-results", title: "Simulation results", valueBasis: "simulated" }
    ]
  };
}
