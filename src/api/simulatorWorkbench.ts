import type { WorkbenchDataAdapter } from "../domain/simulator-workbench/projection";

export type WorkbenchValueBasis = "measured" | "imputed" | "simulated";
export type AvailabilityPhase =
  | "online generation"
  | "ramping"
  | "cooldown"
  | "planned maintenance outage"
  | "unplanned maintenance outage"
  | "refueling outage";
export type CommercialMode =
  | "PPA electric"
  | "direct unit sale"
  | "facility heat"
  | "desalination heat"
  | "resilience backup";
export type TwinViewportEntity =
  | "core"
  | "controlDrums"
  | "primaryLoop"
  | "heatExchangers"
  | "circulators"
  | "vessel"
  | "shielding"
  | "containerBoundary"
  | "secondaryHeatUse"
  | "powerConversion";

export type WorkbenchFreshness = {
  ageSec: number;
  status: "fresh" | "stale" | "degraded" | "unknown";
};

export type WorkbenchValue = {
  valueId: string;
  label: string;
  valueBasis: WorkbenchValueBasis;
  unit: string;
  value: Record<string, unknown>;
  confidence: number;
  freshness: WorkbenchFreshness;
  lineageId: string;
  sourceIds: string[];
};

export type MeasuredTelemetryFrame = {
  schemaVersion: "scada.telemetry.v1";
  sourceId: string;
  tagId: string;
  assetId: string;
  signalKind: "flux" | "temperature" | "pressure" | "flow" | "actuatorState" | "electricalState" | "comms";
  sampledAt: string;
  observedAt: string;
  sequence: number;
  unit: string;
  value: Record<string, unknown>;
  quality: "good" | "stale" | "bad" | "missing" | "estimated";
  valueBasis: "measured";
  syntheticStatus: "public-safe-standin";
};

export type DigitalTwinState = {
  schemaVersion: "digital-twin.state.v1";
  twinId: string;
  asOf: string;
  entities: Array<{
    entityId: string;
    unitId: string;
    displayName: string;
    viewportEntity: TwinViewportEntity;
    values: WorkbenchValue[];
  }>;
};

export type WorkbenchLineage = {
  schemaVersion: "digital-twin.lineage.v1";
  lineageId: string;
  valueId: string;
  valueBasis: WorkbenchValueBasis;
  inputs: Array<{
    sourceKind: "scada-tag" | "digital-twin-model" | "simulation-run" | "artifact";
    sourceId: string;
    valueBasis: WorkbenchValueBasis;
  }>;
  processingSteps: string[];
  artifacts: Array<{
    artifactId: string;
    path: string;
    mediaType: string;
  }>;
};

export type SimulatorWorkbenchState = {
  schemaVersion: "simulator-workbench.state.v1";
  generatedAt: string;
  scenarioId: string;
  selectedUnitId: string;
  valueBasisSummary: Record<WorkbenchValueBasis, number>;
  measuredStateRefs: string[];
  twinStateRef: string;
  lineageRefs: string[];
  fleetUnitRefs: string[];
  commercialDisplayBasisRefs: string[];
  activeSimulationRuns: Array<{
    runId: string;
    scenarioId: string;
    lifecycle: string;
    valueBasis: "simulated";
    health: string;
    artifactStatus: string;
  }>;
  panels: Array<{
    panelId: string;
    title: string;
    valueBasis: WorkbenchValueBasis;
  }>;
};

export type FleetFreshness = {
  status: "fresh" | "late" | "stale";
  ageSec: number;
};

export type AccruedDisplayValue = {
  compactLabel: string;
  amountKUsd: number | null;
  estimate: boolean;
};

export type KaleidosUnitSummary = {
  unitId: string;
  displayName: string;
  availabilityPhase: AvailabilityPhase;
  commercialMode: CommercialMode;
  breakerToBreakerLabel: string;
  electricOutputMwe: number;
  usefulThermalOutputMwt: number;
  residualHeatMwth: number | null;
  accruedDisplayValue: AccruedDisplayValue;
  freshness: FleetFreshness;
  commercialBasisId: string;
  emphasis: string;
};

export type CommercialDisplayBasis = {
  basisId: string;
  unitId: string;
  commercialMode: CommercialMode;
  displayLabel: "Accrued Display Value";
  displayValue: string;
  outputWindow: string;
  deliveredEnergyMwh: number;
  deliveredHeatMwh: number;
  rateAssumptionLabel: string;
  freshnessTimestamp: string;
  exclusions: string[];
};

const WORKBENCH_API_BASE = (import.meta.env.VITE_SIMULATOR_WORKBENCH_API_BASE ?? "").replace(/\/$/, "");

function workbenchApiUrl(path: string): string {
  return `${WORKBENCH_API_BASE}${path}`;
}

export async function getSimulatorWorkbenchState(): Promise<SimulatorWorkbenchState> {
  return readJsonResponse<SimulatorWorkbenchState>(
    await fetch(workbenchApiUrl("/api/simulator-workbench/state"), {
      method: "GET",
      headers: { Accept: "application/json" }
    })
  );
}

export async function getMeasuredState(): Promise<MeasuredTelemetryFrame[]> {
  return readJsonResponse<MeasuredTelemetryFrame[]>(
    await fetch(workbenchApiUrl("/api/simulator-workbench/measured"), {
      method: "GET",
      headers: { Accept: "application/json" }
    })
  );
}

export async function getTwinState(): Promise<DigitalTwinState> {
  return readJsonResponse<DigitalTwinState>(
    await fetch(workbenchApiUrl("/api/simulator-workbench/twin"), {
      method: "GET",
      headers: { Accept: "application/json" }
    })
  );
}

export async function getWorkbenchLineage(valueId: string): Promise<WorkbenchLineage> {
  return readJsonResponse<WorkbenchLineage>(
    await fetch(workbenchApiUrl(`/api/simulator-workbench/lineage/${encodeURIComponent(valueId)}`), {
      method: "GET",
      headers: { Accept: "application/json" }
    })
  );
}

export const httpWorkbenchDataAdapter: WorkbenchDataAdapter = {
  async load() {
    throw new Error("HTTP Simulator Workbench data adapter is parked for this presentational slice.");
  }
};

async function readJsonResponse<T>(response: Response): Promise<T> {
  const raw = await response.text();
  if (!response.ok) {
    throw new Error(`request failed (${response.status}): ${raw}`);
  }
  return JSON.parse(raw) as T;
}
