import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { fixtures } from "../../domain/readiness";
import {
  buildWorkbenchProjection,
  createWorkbenchSnapshotSession,
  loadFixtureWorkbenchData,
  WorkbenchReadError
} from "../../domain/simulator-workbench";
import type { WorkbenchReadState } from "../../domain/simulator-workbench";
import type { SimulationHealthPanelModel } from "./SimulationHealthPanel";
import { SimulatorWorkbenchSurface } from "./SimulatorWorkbenchSurface";

describe("SimulatorWorkbenchSurface", () => {
  it("renders session-owned unit and value selection across fallback, recovery, stale retention, and refresh", async () => {
    const input = loadFixtureWorkbenchData();
    const load = vi.fn()
      .mockRejectedValueOnce(new WorkbenchReadError("unavailable", "offline"))
      .mockResolvedValueOnce({ generation: 8, source: "live", input })
      .mockRejectedValueOnce(new WorkbenchReadError("unavailable", "offline again"))
      .mockResolvedValueOnce({ generation: 9, source: "live", input });
    const session = createWorkbenchSnapshotSession({ load }, {
      allowFixtureFallback: true,
      fixtureInput: input,
      refreshIntervalMs: 60_000
    });
    await session.start();
    session.getResult().selectUnit("KAL-03", "CDB-KAL-03-DESALINATION");
    session.getResult().selectValue("VAL-KAL-03-MEASURED-ELECTRIC-OUTPUT");

    const renderResult = () => {
      const result = session.getResult();
      return renderToStaticMarkup(
        <SimulatorWorkbenchSurface
          onSelectUnit={result.selectUnit}
          onSelectValue={result.selectValue}
          projection={result.projection!}
          readState={result.readState}
          healthPanelModel={result.readState.model!.healthPanelModel}
          onRefresh={result.refresh}
          computeQueue={<div>Scientific compute queue</div>}
          selectedJob={fixtures.computeJobs[0]}
          scenario="DOME synthetic full-power readiness bundle"
          scenarioJobs={fixtures.computeJobs}
          bundleState="ready"
          orchestrationPanel={<div>Containerized worker orchestration</div>}
        />
      );
    };

    expect(renderResult()).toContain("Fixture fallback");
    expect(renderResult()).toContain("Kaleidos Unit 03");
    expect(renderResult()).toContain("simwb-value selected measured");
    await session.refresh();
    expect(renderResult()).toContain("Live generation 8");
    expect(renderResult()).toContain("Kaleidos Unit 03");
    expect(renderResult()).toContain("simwb-value selected measured");
    await session.refresh();
    expect(renderResult()).toContain("Stale live generation 8");
    expect(renderResult()).toContain("Kaleidos Unit 03");
    expect(renderResult()).toContain("simwb-value selected measured");
    await session.refresh();
    expect(renderResult()).toContain("Live generation 9");
    expect(renderResult()).toContain("Kaleidos Unit 03");
    expect(renderResult()).toContain("simwb-value selected measured");
    session.dispose();
  });

  it("renders the integrated Status Workbench with fleet, viewport, value bases, orchestration, and HPC status", () => {
    const markup = renderToStaticMarkup(
      <SimulatorWorkbenchSurface
        onSelectUnit={vi.fn()}
        onSelectValue={vi.fn()}
        projection={buildWorkbenchProjection(loadFixtureWorkbenchData())}
        readState={fixtureReadState()}
        healthPanelModel={distinctiveLiveHealth()}
        onRefresh={vi.fn()}
        computeQueue={<div>Scientific compute queue</div>}
        selectedJob={fixtures.computeJobs.find((job) => job.id === "JOB-HPC-404") ?? fixtures.computeJobs[0]}
        scenario="DOME synthetic full-power readiness bundle"
        scenarioJobs={fixtures.computeJobs}
        bundleState="ready"
        orchestrationPanel={<div>Containerized worker orchestration</div>}
      />
    );

    expect(markup).toContain("Status Workbench");
    expect(markup).toContain("Fixture fallback");
    expect(markup).toContain("Refresh live Snapshot");
    expect(markup).toContain("KAL-01");
    expect(markup).toContain("KAL-05");
    expect(markup).toContain("Fleet Board");
    expect(markup).toContain("30-day contract sprint");
    expect(markup).toContain("TRISO Supply");
    expect(markup).toContain("Measured State");
    expect(markup).toContain("Imputed State");
    expect(markup).toContain("Simulated Result State");
    expect(markup).toContain("Kaleidos Unit twin topology overlay");
    expect(markup).toContain("Kaleidos Unit public-safe digital twin schematic");
    expect(markup).toContain('id="core"');
    expect(markup).toContain('id="heatExchangers"');
    expect(markup).not.toContain("<img");
    expect(markup).not.toContain("digital-twin-concept-v1.png");
    expect(markup).toContain("Engineering Lineage");
    expect(markup).toContain("Core Power Distribution Estimate");
    expect(markup.toLowerCase()).not.toContain("revenue");
    expect(markup).not.toContain("/api/simulator-workbench");
    expect(markup).toContain("Container orchestration");
    expect(markup).toContain("Containerized worker orchestration");
    expect(markup).toContain("Scientific compute queue");
    expect(markup).toContain("HPC Status Panel");
    expect(markup).toContain("Simulation Health Summary");
    expect(markup).toContain("0/1 live runs complete");
    expect(markup).toContain("1/1 live workers nominal");
    expect(markup).toContain("live-artifact-committed");
    expect(markup).not.toContain("4/4 nominal");
    expect(markup).toContain("Panel 1: Multiphysics Co-scheduler");
    expect(markup).toContain("Panel 2: I/O Checkpoint Burst Buffer");
    expect(markup).toContain("Panel 3: Core Thermal Mesh Cloud Burst");
    expect(markup).toContain("Panel 4: Fabric Topology and MPI Profiler");
    expect(markup).not.toContain("Simulation Health (4-card interpretation)");
  });

  it("renders commercial display basis when a fleet commercial value is selected", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {
      selectedUnitId: "KAL-03",
      selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
    });
    const markup = renderToStaticMarkup(
      <SimulatorWorkbenchSurface
        onSelectUnit={vi.fn()}
        onSelectValue={vi.fn()}
        projection={projection}
        readState={fixtureReadState()}
        healthPanelModel={distinctiveLiveHealth()}
        onRefresh={vi.fn()}
        computeQueue={<div>Scientific compute queue</div>}
        selectedJob={fixtures.computeJobs[0]}
        scenario="DOME synthetic full-power readiness bundle"
        scenarioJobs={fixtures.computeJobs}
        bundleState="ready"
        orchestrationPanel={<div>Containerized worker orchestration</div>}
      />
    );

    expect(markup).toContain("Commercial Display Basis");
    expect(markup).toContain("Accrued Display Value");
    expect(markup).toContain("$18.4k (est)");
    expect(markup).toContain("desalination heat");
    expect(markup).toContain("not billing");
    expect(markup).toContain("not settlement");
    expect(markup).not.toContain("Redpanda");
  });

  it("presents stale retention and recovery as explicit read outcomes", () => {
    const projectionInput = loadFixtureWorkbenchData();
    const projection = buildWorkbenchProjection(projectionInput);
    const model = {
      generation: 8,
      source: "live" as const,
      input: projectionInput,
      acceptedAt: "2026-07-18T12:00:00Z",
      healthPanelModel: distinctiveLiveHealth()
    };
    const renderStatus = (readState: WorkbenchReadState) => renderToStaticMarkup(
      <SimulatorWorkbenchSurface
        onSelectUnit={vi.fn()}
        onSelectValue={vi.fn()}
        projection={projection}
        readState={readState}
        healthPanelModel={readState.model!.healthPanelModel}
        onRefresh={vi.fn()}
        computeQueue={<div>Scientific compute queue</div>}
        selectedJob={fixtures.computeJobs[0]}
        scenario="DOME synthetic full-power readiness bundle"
        scenarioJobs={fixtures.computeJobs}
        bundleState="ready"
        orchestrationPanel={<div>Containerized worker orchestration</div>}
      />
    );

    const stale = renderStatus({
      phase: "stale",
      model,
      message: "Workbench service unavailable. Retaining live generation 8 as stale.",
      errorKind: "unavailable"
    });
    expect(stale).toContain("Stale live generation 8");
    expect(stale).toContain("Retaining live generation 8 as stale");

    const recovering = renderStatus({
      phase: "recovering",
      model,
      message: "Refreshing one coherent live Workbench Snapshot."
    });
    expect(recovering).toContain("Recovering live Snapshot");
    expect(recovering).toContain("Refreshing one coherent live Workbench Snapshot");
  });
});

function fixtureReadState(): WorkbenchReadState {
  return {
    phase: "fixture",
    model: {
      generation: 0,
      source: "fixture",
      input: loadFixtureWorkbenchData(),
      healthPanelModel: distinctiveLiveHealth(),
      acceptedAt: "2026-07-14T12:00:00Z"
    },
    message: "Using the explicit local-demo fixture Snapshot."
  };
}

function distinctiveLiveHealth(): SimulationHealthPanelModel {
  return {
    lifecycle: {
      title: "Lifecycle",
      summary: "0/1 live runs complete",
      detail: "distinctive live lifecycle",
      status: "degraded"
    },
    worker: {
      title: "Worker",
      summary: "1/1 live workers nominal",
      detail: "distinctive live worker",
      status: "healthy"
    },
    artifact: {
      title: "Artifact",
      summary: "1 live artifact",
      detail: "live-artifact-committed",
      status: "healthy"
    },
    streamFreshness: {
      title: "Stream freshness",
      summary: "fresh",
      detail: "distinctive live timestamp",
      status: "healthy"
    }
  };
}
