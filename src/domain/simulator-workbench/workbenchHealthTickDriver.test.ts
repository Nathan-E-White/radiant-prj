import { beforeEach, describe, expect, it, vi } from "vitest";
import { createHealthTickDriver } from "./workbenchHealthTickDriver";

const fixtures = [
  {
    generatedAt: "2026-07-06T15:00:00Z",
    activeSimulationRuns: [
      {
        runId: "RUN-A",
        lifecycle: "complete",
        health: "nominal",
        artifactStatus: "fixture-reference",
        scenarioId: "scheduler-drift"
      }
    ]
  },
  {
    generatedAt: "2026-07-06T15:00:10Z",
    activeSimulationRuns: [
      {
        runId: "RUN-B",
        lifecycle: "running",
        health: "nominal",
        artifactStatus: "partial",
        scenarioId: "thermal-sweep"
      }
    ]
  }
];

describe("workbenchHealthTickDriver", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it("emits immediate initial projection and advances on interval", () => {
    const snapshots: string[] = [];
    const driver = createHealthTickDriver({
      intervalMs: 1000,
      fixtures,
      initialNow: new Date("2026-07-06T15:00:00Z"),
      onTick: (snapshot) => {
        snapshots.push(snapshot.lifecycle.summary);
      }
    });

    expect(snapshots).toHaveLength(1);
    expect(snapshots[0]).toBe("1/1 complete");

    vi.advanceTimersByTime(1000);
    expect(snapshots).toHaveLength(2);
    expect(snapshots[1]).toBe("0/1 complete");

    driver.stop();
    vi.advanceTimersByTime(1000);
    expect(snapshots).toHaveLength(2);
  });

  it("falls back to a safe state when fixtures are missing", () => {
    const snapshots: string[] = [];
    const driver = createHealthTickDriver({
      intervalMs: 1000,
      fixtures: [],
      initialNow: new Date("2026-07-06T15:00:00Z"),
      onTick: (snapshot) => {
        snapshots.push(snapshot.lifecycle.summary);
      }
    });

    expect(snapshots[0]).toBe("No runs");
    driver.stop();
  });
});
