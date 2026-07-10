import type { SimulationHealthCard, SimulationHealthPanelModel, SimulationHealthSeverity } from "../../components/simulator-workbench/SimulationHealthPanel";

export type WorkbenchHealthRunState = {
  runId: string;
  lifecycle: string;
  health: string;
  artifactStatus: string;
  scenarioId?: string;
};

export type WorkbenchStateView = {
  generatedAt: string;
  activeSimulationRuns: Array<WorkbenchHealthRunState>;
};

const lifecycleHealthy = new Set(["complete", "complete+success", "completed"]);
const lifecycleRunning = new Set(["running", "scheduled", "queued", "launching", "starting"]);
const lifecycleCritical = new Set(["failed", "errored", "aborted", "cancelled", "canceled", "crashed"]);
const artifactHealthy = new Set(["planned-reference", "fixture-reference", "committed", "committing", "staged", "reference"]);
const artifactDegraded = new Set(["partial", "stale", "pending", "delayed"]);
const artifactCritical = new Set(["missing", "failed", "blocked", "error", "invalid"]);

export function projectHealthCards(input: WorkbenchStateView, now: Date): SimulationHealthPanelModel {
  const runs = Array.isArray(input.activeSimulationRuns) ? input.activeSimulationRuns : [];
  const nowAt = now instanceof Date && !Number.isNaN(now.getTime()) ? now : new Date();
  const baselineRuns = normalizeWorkRuns(runs);

  return {
    lifecycle: projectLifecycleCard(baselineRuns),
    worker: projectWorkerCard(baselineRuns),
    artifact: projectArtifactCard(baselineRuns),
    streamFreshness: projectStreamFreshnessCard(input.generatedAt, nowAt)
  };
}

function projectLifecycleCard(runs: NormalizedRun[]): SimulationHealthCard {
  if (runs.length === 0) {
    return {
      title: "Lifecycle",
      summary: "No runs",
      status: "stale",
      detail: "No simulation runs are loaded for this state view."
    };
  }

  const completeRuns = runs.filter((run) => run.lifecycle === "healthy" || lifecycleHealthy.has(run.lifecycle)).length;
  const criticalRuns = runs.filter((run) => run.lifecycle === "critical" || lifecycleCritical.has(run.lifecycle)).length;
  const activeRuns = runs.filter((run) => lifecycleRunning.has(run.lifecycle) || run.lifecycle === "active").length;

  let status: SimulationHealthSeverity = "healthy";
  if (criticalRuns > 0) {
    status = "critical";
  } else if (activeRuns > 0 || completeRuns < runs.length) {
    status = "degraded";
  }

  const summary = `${completeRuns}/${runs.length} complete`;
  const detail = detailFromLifecycle(runs, completeRuns, criticalRuns, activeRuns);
  return {
    title: "Lifecycle",
    summary,
    detail,
    status
  };
}

function projectWorkerCard(runs: NormalizedRun[]): SimulationHealthCard {
  if (runs.length === 0) {
    return {
      title: "Worker",
      summary: "No workers",
      status: "stale",
      detail: "No worker snapshots are available."
    };
  }

  const nominalRuns = runs.filter((run) => run.health === "healthy" || run.health === "nominal").length;
  const criticalRuns = runs.filter((run) => run.health === "critical" || run.health === "failed").length;
  const degradedRuns = runs.length - nominalRuns - criticalRuns;

  let status: SimulationHealthSeverity = "healthy";
  if (criticalRuns > 0) status = "critical";
  else if (degradedRuns > 0) status = "degraded";

  const summary = `${nominalRuns}/${runs.length} nominal`;
  const detail = detailFromWorker(runs, criticalRuns, degradedRuns);
  return { title: "Worker", summary, detail, status };
}

function projectArtifactCard(runs: NormalizedRun[]): SimulationHealthCard {
  if (runs.length === 0) {
    return {
      title: "Artifact",
      summary: "No artifacts",
      status: "stale",
      detail: "No artifact statuses are currently available."
    };
  }

  const artifactStatuses = runs.map((run) => run.artifactStatus);
  const uniqueStatuses = Array.from(new Set(artifactStatuses));
  const criticalCount = artifactStatuses.filter((status) => artifactCritical.has(status)).length;
  const degradedCount = artifactStatuses.filter(
    (status) => artifactDegraded.has(status) || (artifactCritical.has(status) === false && artifactHealthy.has(status) === false)
  ).length;

  let status: SimulationHealthSeverity = "healthy";
  if (criticalCount > 0) status = "critical";
  else if (degradedCount > 0) status = "degraded";

  const summary = `${artifactStatuses.length} artifact statuses`;
  const detail = uniqueStatuses.join(", ");
  return { title: "Artifact", summary, detail, status };
};

function projectStreamFreshnessCard(generatedAt: string, now: Date): SimulationHealthCard {
  const timestamp = new Date(generatedAt);
  if (generatedAt === "" || Number.isNaN(timestamp.getTime())) {
    return {
      title: "Stream freshness",
      summary: "No stream timestamp",
      status: "stale",
      detail: "No stream timestamp available for this view."
    };
  }

  const ageSeconds = Math.max(0, Math.floor((now.getTime() - timestamp.getTime()) / 1000));
  let status: SimulationHealthSeverity = "healthy";
  if (ageSeconds > 300) status = "critical";
  else if (ageSeconds > 60) status = "stale";

  return {
    title: "Stream freshness",
    summary: ageDescription(ageSeconds),
    detail: `Latest projection generated ${ageSeconds}s ago`,
    status
  };
}

function ageDescription(seconds: number): string {
  if (seconds <= 15) return "fresh";
  if (seconds <= 120) return "aging";
  if (seconds <= 300) return "stale";
  return "critical";
}

function detailFromLifecycle(runs: NormalizedRun[], complete: number, critical: number, active: number): string {
  if (critical > 0) {
    return "Critical lifecycle states require operator review.";
  }
  if (active > 0) {
    return `${active} runs in flight while ${complete} complete`;
  }
  if (complete < runs.length) {
    return `${runs.length - complete} runs not yet complete`;
  }
  return "All lifecycle states are complete";
}

function detailFromWorker(runs: NormalizedRun[], critical: number, degraded: number): string {
  if (critical > 0) {
    return `${critical} runs report non-nominal worker health`;
  }
  if (degraded > 0) {
    const degradedRuns = runs.filter((run) => run.health !== "healthy" && run.health !== "nominal" && run.health !== "critical" && run.health !== "failed");
    return `${degradedRuns.length} runs in degraded state`;
  }
  const scenarioCount = new Set(runs.map((run) => run.scenarioId).filter(Boolean)).size;
  return `${scenarioCount} scenario${scenarioCount === 1 ? "" : "s"} represented`;
}

type NormalizedRun = {
  lifecycle: string;
  health: string;
  artifactStatus: string;
  scenarioId?: string;
};

function normalizeWorkRuns(runs: WorkbenchHealthRunState[]): NormalizedRun[] {
  return runs.map((run) => ({
    lifecycle: String(run.lifecycle ?? "").toLowerCase(),
    health: String(run.health ?? "").toLowerCase(),
    artifactStatus: String(run.artifactStatus ?? "").toLowerCase(),
    scenarioId: run.scenarioId
  }));
}
