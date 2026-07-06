import type {
  DigitalTwinState,
  MeasuredTelemetryFrame,
  SimulatorWorkbenchState,
  WorkbenchLineage,
  WorkbenchValue,
  WorkbenchValueBasis
} from "../../api/simulatorWorkbench";
import { workbenchValueBasisOrder, valueBasisLabel } from "./valueBasis";

export type WorkbenchProjectionInput = {
  state: SimulatorWorkbenchState;
  measured: MeasuredTelemetryFrame[];
  twin: DigitalTwinState;
  lineages: WorkbenchLineage[];
};

export type WorkbenchDataAdapter = {
  load(): Promise<WorkbenchProjectionInput>;
};

export type ProjectedWorkbenchValue = WorkbenchValue & {
  entityId: string;
  entityName: string;
  displayValue: string;
  confidencePct: number;
  freshnessLabel: string;
  sourceQuality: string;
  lineage?: WorkbenchLineage;
};

export type WorkbenchBasisGroup = {
  basis: WorkbenchValueBasis;
  label: string;
  count: number;
  values: ProjectedWorkbenchValue[];
  panelTitle: string;
};

export type WorkbenchLineageStep = {
  id: string;
  label: string;
  detail: string;
  basis?: WorkbenchValueBasis;
};

export type WorkbenchHealthSummary = {
  scope: "summary";
  status: "complete" | "degraded" | "pending";
  label: string;
  detail: string;
  runCount: number;
  completedRuns: number;
  artifactStatuses: string[];
};

export type WorkbenchProjection = {
  generatedAt: string;
  scenarioId: string;
  twinId: string;
  groups: Record<WorkbenchValueBasis, WorkbenchBasisGroup>;
  panelSummaries: SimulatorWorkbenchState["panels"];
  measuredFrames: MeasuredTelemetryFrame[];
  healthSummary: WorkbenchHealthSummary;
  selectedValue: ProjectedWorkbenchValue | null;
  selectedLineage: WorkbenchLineage | null;
  selectedLineageMissing: boolean;
  lineageSteps: WorkbenchLineageStep[];
  defaultSelectedValueId: string | null;
  valueBasisSummary: Record<WorkbenchValueBasis, number>;
};

export function buildWorkbenchProjection(
  input: WorkbenchProjectionInput,
  selectedValueId?: string
): WorkbenchProjection {
  const lineagesByValue = new Map(input.lineages.map((lineage) => [lineage.valueId, lineage]));
  const latestMeasuredQuality = latestMeasuredQualityByTag(input.measured);
  const values = input.twin.entities.flatMap((entity) =>
    entity.values.map<ProjectedWorkbenchValue>((value) => ({
      ...value,
      entityId: entity.entityId,
      entityName: entity.displayName,
      displayValue: formatWorkbenchValue(value.value),
      confidencePct: Math.round(value.confidence * 100),
      freshnessLabel: formatFreshness(value.freshness.ageSec, value.freshness.status),
      sourceQuality: summarizeSourceQuality(value.sourceIds, latestMeasuredQuality),
      lineage: lineagesByValue.get(value.valueId)
    }))
  );

  const requestedValue = selectedValueId ? values.find((value) => value.valueId === selectedValueId) : undefined;
  const defaultSelectedValue = values.find((value) => value.valueBasis === "imputed") ?? values[0] ?? null;
  const selectedValue = requestedValue ?? defaultSelectedValue;
  const selectedLineage = selectedValue?.lineage ?? null;
  const selectedLineageMissing = Boolean(selectedValue && !selectedLineage);
  const valueBasisSummary = summarizeProjectedValues(values);

  return {
    generatedAt: input.state.generatedAt,
    scenarioId: input.state.scenarioId,
    twinId: input.twin.twinId,
    groups: groupValues(values),
    panelSummaries: input.state.panels,
    measuredFrames: input.measured,
    healthSummary: buildHealthSummary(input.state),
    selectedValue,
    selectedLineage,
    selectedLineageMissing,
    lineageSteps: selectedLineage ? buildLineageSteps(selectedLineage) : [],
    defaultSelectedValueId: defaultSelectedValue?.valueId ?? null,
    valueBasisSummary
  };
}

function groupValues(values: ProjectedWorkbenchValue[]): Record<WorkbenchValueBasis, WorkbenchBasisGroup> {
  return workbenchValueBasisOrder.reduce<Record<WorkbenchValueBasis, WorkbenchBasisGroup>>(
    (groups, basis) => {
      const basisValues = values.filter((value) => value.valueBasis === basis);
      groups[basis] = {
        basis,
        label: valueBasisLabel(basis),
        count: basisValues.length,
        values: basisValues,
        panelTitle: `${valueBasisLabel(basis)} state`
      };
      return groups;
    },
    {
      measured: emptyGroup("measured"),
      imputed: emptyGroup("imputed"),
      simulated: emptyGroup("simulated")
    }
  );
}

function emptyGroup(basis: WorkbenchValueBasis): WorkbenchBasisGroup {
  return {
    basis,
    label: valueBasisLabel(basis),
    count: 0,
    values: [],
    panelTitle: `${valueBasisLabel(basis)} state`
  };
}

function summarizeProjectedValues(values: ProjectedWorkbenchValue[]): Record<WorkbenchValueBasis, number> {
  return values.reduce<Record<WorkbenchValueBasis, number>>(
    (summary, value) => {
      summary[value.valueBasis] += 1;
      return summary;
    },
    { measured: 0, imputed: 0, simulated: 0 }
  );
}

function buildHealthSummary(state: SimulatorWorkbenchState): WorkbenchHealthSummary {
  const runs = state.activeSimulationRuns;
  const completedRuns = runs.filter((run) => run.lifecycle === "complete").length;
  const artifactStatuses = Array.from(new Set(runs.map((run) => run.artifactStatus)));
  const degraded = runs.some((run) => run.health !== "nominal" || run.lifecycle === "failed");
  const pending = runs.some((run) => run.lifecycle !== "complete" && run.lifecycle !== "failed" && run.lifecycle !== "stopped");

  if (degraded) {
    return {
      scope: "summary",
      status: "degraded",
      label: "Review",
      detail: `${completedRuns}/${runs.length} runs complete`,
      runCount: runs.length,
      completedRuns,
      artifactStatuses
    };
  }

  if (pending) {
    return {
      scope: "summary",
      status: "pending",
      label: "Pending",
      detail: `${completedRuns}/${runs.length} runs complete`,
      runCount: runs.length,
      completedRuns,
      artifactStatuses
    };
  }

  return {
    scope: "summary",
    status: "complete",
    label: "Complete",
    detail: `${completedRuns}/${runs.length} runs complete`,
    runCount: runs.length,
    completedRuns,
    artifactStatuses
  };
}

function buildLineageSteps(lineage: WorkbenchLineage): WorkbenchLineageStep[] {
  const inputSteps = lineage.inputs.map<WorkbenchLineageStep>((input, index) => ({
    id: `${lineage.lineageId}-input-${index}`,
    label: input.sourceKind,
    detail: input.sourceId,
    basis: input.valueBasis
  }));
  const processingSteps = lineage.processingSteps.map<WorkbenchLineageStep>((step, index) => ({
    id: `${lineage.lineageId}-process-${index}`,
    label: "processing",
    detail: step,
    basis: lineage.valueBasis
  }));
  const artifactSteps = lineage.artifacts.map<WorkbenchLineageStep>((artifact) => ({
    id: `${lineage.lineageId}-artifact-${artifact.artifactId}`,
    label: "artifact",
    detail: artifact.artifactId,
    basis: lineage.valueBasis
  }));
  return [...inputSteps, ...processingSteps, ...artifactSteps];
}

function latestMeasuredQualityByTag(frames: MeasuredTelemetryFrame[]): Map<string, MeasuredTelemetryFrame["quality"]> {
  const latest = new Map<string, MeasuredTelemetryFrame>();
  for (const frame of frames) {
    const existing = latest.get(frame.tagId);
    if (!existing || frame.sequence >= existing.sequence) {
      latest.set(frame.tagId, frame);
    }
  }
  return new Map(Array.from(latest.entries()).map(([tagId, frame]) => [tagId, frame.quality]));
}

function summarizeSourceQuality(
  sourceIds: string[],
  qualityByTag: Map<string, MeasuredTelemetryFrame["quality"]>
): string {
  const qualities = sourceIds.flatMap((sourceId) => {
    const quality = qualityByTag.get(sourceId);
    return quality ? [quality] : [];
  });
  if (qualities.length === 0) {
    return "model-linked";
  }
  if (qualities.every((quality) => quality === "good")) {
    return "good";
  }
  if (qualities.some((quality) => quality === "bad" || quality === "missing")) {
    return "degraded";
  }
  return "watch";
}

function formatFreshness(ageSec: number, status: string): string {
  return `${status} / ${ageSec}s`;
}

function formatWorkbenchValue(value: Record<string, unknown>): string {
  if (typeof value.scalar === "number") {
    return formatNumber(value.scalar);
  }
  if (typeof value.state === "string") {
    return value.state;
  }
  if (typeof value.band === "string") {
    return value.band;
  }
  const firstEntry = Object.entries(value)[0];
  if (!firstEntry) {
    return "n/a";
  }
  const [, raw] = firstEntry;
  if (typeof raw === "number") {
    return formatNumber(raw);
  }
  if (typeof raw === "boolean") {
    return raw ? "true" : "false";
  }
  return String(raw);
}

function formatNumber(value: number): string {
  if (Math.abs(value) >= 100) {
    return value.toFixed(1);
  }
  if (Math.abs(value) >= 10) {
    return value.toFixed(1);
  }
  return value.toFixed(2);
}
