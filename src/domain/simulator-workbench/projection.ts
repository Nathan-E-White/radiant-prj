import type {
  CommercialDisplayBasis,
  DigitalTwinState,
  KaleidosUnitSummary,
  MeasuredTelemetryFrame,
  SimulatorWorkbenchState,
  TwinViewportEntity,
  WorkbenchLineage,
  WorkbenchValue,
  WorkbenchValueBasis
} from "../../api/simulatorWorkbench";
import { valueBasisLabel, workbenchValueBasisOrder } from "./valueBasis";

export type WorkbenchProjectionInput = {
  state: SimulatorWorkbenchState;
  measured: MeasuredTelemetryFrame[];
  twin: DigitalTwinState;
  lineages: WorkbenchLineage[];
  fleetUnits: KaleidosUnitSummary[];
  commercialDisplayBasis: CommercialDisplayBasis[];
};

export type WorkbenchDataAdapter = {
  load(): Promise<WorkbenchProjectionInput>;
};

export type WorkbenchSelection = {
  selectedUnitId?: string;
  selectedValueId?: string;
  selectedCommercialBasisId?: string;
};

export type ProjectedWorkbenchValue = WorkbenchValue & {
  entityId: string;
  entityName: string;
  unitId: string;
  viewportEntity: TwinViewportEntity;
  displayValue: string;
  confidencePct: number;
  freshnessLabel: string | null;
  sourceQuality: string;
  lineage?: WorkbenchLineage;
};

export type ProjectedFleetUnit = KaleidosUnitSummary & {
  selected: boolean;
  phaseLine: string;
  outputLine: string;
  accruedDisplayLabel: string;
  freshnessWarningLabel: string | null;
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

export type TwinViewportLayer = {
  id: string;
  unitId: string;
  entityId: TwinViewportEntity;
  valueId: string;
  label: string;
  valueBasis: WorkbenchValueBasis;
  selected: boolean;
  confidencePct: number;
};

export type TwinViewportModel = {
  selectedUnitId: string;
  selectedEntity: TwinViewportEntity;
  layers: TwinViewportLayer[];
  entityIds: TwinViewportEntity[];
};

export type WorkbenchExplanation = {
  kind: "engineering" | "commercial";
  title: string;
  subtitle: string;
  basisLabel: string;
  items: Array<{ label: string; value: string }>;
  exclusions: string[];
  steps: WorkbenchLineageStep[];
};

export type WorkbenchProjection = {
  generatedAt: string;
  scenarioId: string;
  twinId: string;
  selectedUnit: ProjectedFleetUnit;
  fleetUnits: ProjectedFleetUnit[];
  groups: Record<WorkbenchValueBasis, WorkbenchBasisGroup>;
  panelSummaries: SimulatorWorkbenchState["panels"];
  measuredFrames: MeasuredTelemetryFrame[];
  healthSummary: WorkbenchHealthSummary;
  viewport: TwinViewportModel;
  selectedValue: ProjectedWorkbenchValue | null;
  selectedLineage: WorkbenchLineage | null;
  selectedLineageMissing: boolean;
  lineageSteps: WorkbenchLineageStep[];
  explanation: WorkbenchExplanation;
  defaultSelectedValueId: string | null;
  valueBasisSummary: Record<WorkbenchValueBasis, number>;
};

const viewportEntityIds: TwinViewportEntity[] = [
  "core",
  "controlDrums",
  "primaryLoop",
  "heatExchangers",
  "circulators",
  "vessel",
  "shielding",
  "containerBoundary",
  "secondaryHeatUse",
  "powerConversion"
];

const panelTitles: Record<WorkbenchValueBasis, string> = {
  measured: "Measured State",
  imputed: "Imputed State",
  simulated: "Simulated Result State"
};

export function buildWorkbenchProjection(
  input: WorkbenchProjectionInput,
  selection: string | WorkbenchSelection = {}
): WorkbenchProjection {
  const normalizedSelection = normalizeSelection(selection);
  const selectedUnitId = selectUnitId(input, normalizedSelection.selectedUnitId);
  const selectedMeasuredFrames = input.measured.filter((frame) => frame.assetId.startsWith(`${selectedUnitId}-`));
  const lineagesByValue = new Map(input.lineages.map((lineage) => [lineage.valueId, lineage]));
  const latestMeasuredQuality = latestMeasuredQualityByTag(selectedMeasuredFrames);
  const allValues = projectValues(input, latestMeasuredQuality, lineagesByValue);
  const selectedUnitValues = allValues
    .filter((value) => value.unitId === selectedUnitId)
    .filter((value) => canDisplayValue(value, selectedMeasuredFrames));
  const requestedValue = normalizedSelection.selectedValueId
    ? selectedUnitValues.find((value) => value.valueId === normalizedSelection.selectedValueId)
    : undefined;
  const defaultSelectedValue = selectedUnitValues.find((value) => value.valueBasis === "imputed") ?? selectedUnitValues[0] ?? null;
  const selectedValue = requestedValue ?? defaultSelectedValue;
  const selectedLineage = selectedValue?.lineage ?? null;
  const selectedLineageMissing = Boolean(selectedValue && !selectedLineage);
  const lineageSteps = selectedLineage ? buildLineageSteps(selectedLineage) : [];
  const projectedFleetUnits = projectFleetUnits(input.fleetUnits, selectedUnitId);
  const selectedUnit = projectedFleetUnits.find((unit) => unit.unitId === selectedUnitId) ?? projectedFleetUnits[0];
  const selectedCommercialBasis = normalizedSelection.selectedCommercialBasisId
    ? input.commercialDisplayBasis.find(
        (basis) => basis.basisId === normalizedSelection.selectedCommercialBasisId && basis.unitId === selectedUnitId
      )
    : undefined;
  const valueBasisSummary = summarizeProjectedValues(selectedUnitValues);

  return {
    generatedAt: input.state.generatedAt,
    scenarioId: input.state.scenarioId,
    twinId: input.twin.twinId,
    selectedUnit,
    fleetUnits: projectedFleetUnits,
    groups: groupValues(selectedUnitValues),
    panelSummaries: input.state.panels,
    measuredFrames: selectedMeasuredFrames,
    healthSummary: buildHealthSummary(input.state),
    viewport: buildViewportModel(selectedUnitId, selectedUnitValues, selectedValue),
    selectedValue,
    selectedLineage,
    selectedLineageMissing,
    lineageSteps,
    explanation: selectedCommercialBasis
      ? buildCommercialExplanation(selectedCommercialBasis)
      : buildEngineeringExplanation(selectedValue, selectedLineage, lineageSteps, selectedLineageMissing),
    defaultSelectedValueId: defaultSelectedValue?.valueId ?? null,
    valueBasisSummary
  };
}

function normalizeSelection(selection: string | WorkbenchSelection): WorkbenchSelection {
  return typeof selection === "string" ? { selectedValueId: selection } : selection;
}

function selectUnitId(input: WorkbenchProjectionInput, requestedUnitId?: string): string {
  if (requestedUnitId && input.fleetUnits.some((unit) => unit.unitId === requestedUnitId)) {
    return requestedUnitId;
  }
  if (input.state.selectedUnitId && input.fleetUnits.some((unit) => unit.unitId === input.state.selectedUnitId)) {
    return input.state.selectedUnitId;
  }
  return input.fleetUnits[0]?.unitId ?? "";
}

function projectValues(
  input: WorkbenchProjectionInput,
  latestMeasuredQuality: Map<string, MeasuredTelemetryFrame["quality"]>,
  lineagesByValue: Map<string, WorkbenchLineage>
): ProjectedWorkbenchValue[] {
  return input.twin.entities.flatMap((entity) =>
    entity.values.map<ProjectedWorkbenchValue>((value) => ({
      ...value,
      entityId: entity.entityId,
      entityName: entity.displayName,
      unitId: entity.unitId,
      viewportEntity: entity.viewportEntity,
      displayValue: formatWorkbenchValue(value.value),
      confidencePct: Math.round(value.confidence * 100),
      freshnessLabel: formatFreshness(value.freshness.ageSec, value.freshness.status),
      sourceQuality: summarizeSourceQuality(value.sourceIds, latestMeasuredQuality),
      lineage: lineagesByValue.get(value.valueId)
    }))
  );
}

function projectFleetUnits(units: KaleidosUnitSummary[], selectedUnitId: string): ProjectedFleetUnit[] {
  return units.map((unit) => ({
    ...unit,
    selected: unit.unitId === selectedUnitId,
    phaseLine: `${unit.availabilityPhase} | ${unit.breakerToBreakerLabel}`,
    outputLine: formatFleetOutput(unit),
    accruedDisplayLabel: unit.accruedDisplayValue.compactLabel,
    freshnessWarningLabel: formatFleetFreshness(unit.freshness.status, unit.freshness.ageSec)
  }));
}

function formatFleetOutput(unit: KaleidosUnitSummary): string {
  if (unit.residualHeatMwth && unit.residualHeatMwth > 0 && unit.electricOutputMwe === 0 && unit.usefulThermalOutputMwt === 0) {
    return `${formatNumber(unit.residualHeatMwth)} MWth residual heat`;
  }
  if (unit.electricOutputMwe > 0 || unit.usefulThermalOutputMwt > 0) {
    return `${formatNumber(unit.electricOutputMwe)} MWe | ${formatNumber(unit.usefulThermalOutputMwt)} MWt`;
  }
  return unit.breakerToBreakerLabel;
}

function formatFleetFreshness(status: KaleidosUnitSummary["freshness"]["status"], ageSec: number): string | null {
  if (status === "fresh") {
    return null;
  }
  return `${status} ${Math.max(1, Math.round(ageSec / 60))}m`;
}

function canDisplayValue(value: ProjectedWorkbenchValue, measuredFrames: MeasuredTelemetryFrame[]): boolean {
  if (value.label !== "Core Power Distribution Estimate") {
    return true;
  }
  const measuredFluxTags = new Set(
    measuredFrames.filter((frame) => frame.signalKind === "flux" && frame.valueBasis === "measured").map((frame) => frame.tagId)
  );
  return value.sourceIds.filter((sourceId) => measuredFluxTags.has(sourceId)).length >= 2;
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
        panelTitle: panelTitles[basis]
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
    panelTitle: panelTitles[basis]
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
      detail: `${completedRuns}/${runs.length} results complete`,
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
      detail: `${completedRuns}/${runs.length} results complete`,
      runCount: runs.length,
      completedRuns,
      artifactStatuses
    };
  }

  return {
    scope: "summary",
    status: "complete",
    label: "Complete",
    detail: `${completedRuns}/${runs.length} results complete`,
    runCount: runs.length,
    completedRuns,
    artifactStatuses
  };
}

function buildViewportModel(
  selectedUnitId: string,
  values: ProjectedWorkbenchValue[],
  selectedValue: ProjectedWorkbenchValue | null
): TwinViewportModel {
  const selectedEntity = selectedValue?.viewportEntity ?? "core";
  return {
    selectedUnitId,
    selectedEntity,
    entityIds: viewportEntityIds,
    layers: values.map((value) => ({
      id: `${value.valueId}-${value.viewportEntity}`,
      unitId: value.unitId,
      entityId: value.viewportEntity,
      valueId: value.valueId,
      label: value.label,
      valueBasis: value.valueBasis,
      selected: value.valueId === selectedValue?.valueId,
      confidencePct: value.confidencePct
    }))
  };
}

function buildEngineeringExplanation(
  selectedValue: ProjectedWorkbenchValue | null,
  lineage: WorkbenchLineage | null,
  steps: WorkbenchLineageStep[],
  lineageMissing: boolean
): WorkbenchExplanation {
  if (!selectedValue) {
    return {
      kind: "engineering",
      title: "Engineering Lineage",
      subtitle: "No value selected",
      basisLabel: "n/a",
      items: [],
      exclusions: [],
      steps: []
    };
  }

  return {
    kind: "engineering",
    title: "Engineering Lineage",
    subtitle: lineageMissing ? `Lineage pending for ${selectedValue.valueId}` : selectedValue.label,
    basisLabel: selectedValue.valueBasis,
    items: [
      { label: "Value Basis", value: selectedValue.valueBasis },
      { label: "Display value", value: `${selectedValue.displayValue} ${selectedValue.unit}` },
      { label: "Confidence", value: `${selectedValue.confidencePct}%` },
      { label: "Entity", value: selectedValue.entityName }
    ],
    exclusions: lineage ? [] : ["lineage pending"],
    steps
  };
}

function buildCommercialExplanation(basis: CommercialDisplayBasis): WorkbenchExplanation {
  return {
    kind: "commercial",
    title: basis.displayLabel,
    subtitle: "Commercial Display Basis",
    basisLabel: basis.commercialMode,
    items: [
      { label: "Commercial mode", value: basis.commercialMode },
      { label: "Output window", value: basis.outputWindow },
      { label: "Delivered energy", value: `${formatNumber(basis.deliveredEnergyMwh)} MWhe` },
      { label: "Delivered heat", value: `${formatNumber(basis.deliveredHeatMwh)} MWhth` },
      { label: "Rate assumption", value: basis.rateAssumptionLabel },
      { label: "Freshness timestamp", value: basis.freshnessTimestamp },
      { label: "Display value", value: basis.displayValue }
    ],
    exclusions: basis.exclusions,
    steps: []
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

function formatFreshness(ageSec: number, status: string): string | null {
  if (status === "fresh") {
    return null;
  }
  if (ageSec >= 60) {
    return `${status} ${Math.round(ageSec / 60)}m`;
  }
  return `${status} ${ageSec}s`;
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
  if (Number.isInteger(value)) {
    return String(value);
  }
  const fixed = Math.abs(value) < 1 ? value.toFixed(2) : value.toFixed(1);
  return fixed.replace(/\.0$/, "");
}
