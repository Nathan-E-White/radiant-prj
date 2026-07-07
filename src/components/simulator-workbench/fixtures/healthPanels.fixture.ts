import type { SimulationHealthCard, SimulationHealthPanelModel } from "../SimulationHealthPanel";

export const baseSimulationHealthPanelModel: SimulationHealthPanelModel = {
  lifecycle: {
    title: "Lifecycle",
    summary: "2/2 complete",
    status: "healthy",
    detail: "Both scheduler-drift and thermal-sweep runs reached complete"
  },
  artifact: {
    title: "Artifact",
    summary: "all committed",
    status: "healthy",
    detail: "Expected result and lineage artifacts present"
  },
  worker: {
    title: "Worker",
    summary: "4/4 nominal",
    status: "healthy",
    detail: "4 active workers, no hard failures"
  },
  streamFreshness: {
    title: "Stream",
    summary: "fresh",
    status: "healthy",
    detail: "Latest stream sample within 30s"
  }
};

export type SimulationHealthCardOverrides = {
  [K in keyof SimulationHealthPanelModel]?: Partial<SimulationHealthCard>;
};

export function buildSimulationHealthPanelModel(
  overrides: SimulationHealthCardOverrides = {}
): SimulationHealthPanelModel {
  return {
    lifecycle: {
      ...baseSimulationHealthPanelModel.lifecycle,
      ...overrides.lifecycle
    },
    artifact: {
      ...baseSimulationHealthPanelModel.artifact,
      ...overrides.artifact
    },
    worker: {
      ...baseSimulationHealthPanelModel.worker,
      ...overrides.worker
    },
    streamFreshness: {
      ...baseSimulationHealthPanelModel.streamFreshness,
      ...overrides.streamFreshness
    }
  };
}

export const simulationHealthPanelNominal = buildSimulationHealthPanelModel();

export const simulationHealthPanelLifecycleRunningWithStaleStream: SimulationHealthPanelModel =
  buildSimulationHealthPanelModel({
    lifecycle: {
      summary: "1 running / 1 complete",
      status: "degraded",
      detail: "One run still in streaming phase"
    },
    streamFreshness: {
      summary: "stale",
      status: "stale",
      detail: "Latest sample age exceeds freshness threshold"
    }
  });

export const simulationHealthPanelArtifactPipelineDegraded: SimulationHealthPanelModel = buildSimulationHealthPanelModel({
  artifact: {
    summary: "1 missing",
    status: "degraded",
    detail: "Lineage artifact for thermal-sweep unavailable"
  },
  worker: {
    summary: "3 nominal / 1 retrying",
    status: "degraded",
    detail: "One worker restarted after transient fault"
  }
});

export const simulationHealthPanelCriticalWorkerAndArtifacts: SimulationHealthPanelModel = buildSimulationHealthPanelModel({
  lifecycle: {
    summary: "1 failed / 1 running",
    status: "critical",
    detail: "A scheduler run terminated unexpectedly"
  },
  artifact: {
    summary: "blocked",
    status: "critical",
    detail: "Result artifact write path reported partial failure"
  },
  worker: {
    summary: "1 critical",
    status: "critical",
    detail: "One worker exceeded compute budget and exited"
  },
  streamFreshness: {
    summary: "stale",
    status: "critical",
    detail: "No stream heartbeat for > 5 minutes"
  }
});
