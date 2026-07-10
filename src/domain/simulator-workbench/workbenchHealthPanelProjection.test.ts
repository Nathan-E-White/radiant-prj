import { describe, expect, it } from "vitest";
import {
  projectHealthCards,
  type WorkbenchHealthRunState,
  type WorkbenchStateView
} from "./workbenchHealthPanelProjection";

function buildRun(run: Partial<WorkbenchHealthRunState>): WorkbenchHealthRunState {
  return {
    runId: "RUN-001",
    lifecycle: "complete",
    health: "nominal",
    artifactStatus: "fixture-reference",
    scenarioId: "default",
    ...run
  };
}

function fixtureState(runs: WorkbenchHealthRunState[], generatedAt: string): WorkbenchStateView {
  return {
    generatedAt,
    activeSimulationRuns: runs
  };
}

describe("workbenchHealthPanelProjection", () => {
  it("returns exactly four cards with deterministic lifecycle/worker summaries", () => {
    const projection = projectHealthCards(
      fixtureState(
        [
          buildRun({ runId: "RUN-A", scenarioId: "scheduler-drift" }),
          buildRun({ runId: "RUN-B", lifecycle: "running", scenarioId: "thermal-sweep" })
        ],
        "2026-07-06T15:00:00Z"
      ),
      new Date("2026-07-06T15:00:30Z")
    );

    expect(Object.keys(projection).sort()).toEqual(["artifact", "lifecycle", "streamFreshness", "worker"]);
    expect(projection.lifecycle.summary).toBe("1/2 complete");
    expect(projection.lifecycle.status).toBe("degraded");
    expect(projection.worker.summary).toBe("2/2 nominal");
    expect(projection.worker.status).toBe("healthy");
    expect(projection.artifact.status).toBe("healthy");
    expect(projection.streamFreshness.summary).toBe("aging");
  });

  it("elevates lifecycle and worker cards when critical states appear", () => {
    const projection = projectHealthCards(
      fixtureState(
        [
          buildRun({ runId: "RUN-A", lifecycle: "failed", health: "critical", artifactStatus: "missing" }),
          buildRun({ runId: "RUN-B", lifecycle: "failed", health: "failed", artifactStatus: "failed" })
        ],
        "2026-07-06T15:00:00Z"
      ),
      new Date("2026-07-06T15:10:00Z")
    );

    expect(projection.lifecycle.status).toBe("critical");
    expect(projection.worker.status).toBe("critical");
    expect(projection.artifact.status).toBe("critical");
    expect(projection.streamFreshness.status).toBe("critical");
  });

  it("gracefully handles missing run and timestamp values", () => {
    const projection = projectHealthCards({ generatedAt: "", activeSimulationRuns: [] }, new Date("2026-07-06T15:00:00Z"));

    expect(projection.lifecycle.summary).toBe("No runs");
    expect(projection.lifecycle.status).toBe("stale");
    expect(projection.worker.status).toBe("stale");
    expect(projection.artifact.status).toBe("stale");
    expect(projection.streamFreshness.status).toBe("stale");
  });
});
