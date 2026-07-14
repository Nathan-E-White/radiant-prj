import type {
  DigitalTwinState,
  KaleidosUnitSummary,
  MeasuredTelemetryFrame,
  SimulatorWorkbenchState,
  TwinViewportEntity,
  WorkbenchLineage,
  WorkbenchValue,
  WorkbenchValueBasis
} from "../../api/simulatorWorkbench";
import type { WorkbenchProjectionInput } from "./projection";

export type LiveSimopsResultFrame = {
  schemaVersion: "simops.result.v1";
  runId: string;
  scenarioId: string;
  workerId: string;
  workerKind: string;
  sequence: number;
  producedAt: string;
  resultType: string;
  modelId: string;
  inputWindow: { start: string; end: string };
  valueBasis: "simulated";
  syntheticStatus: "public-safe-standin";
  values: Array<{
    resultId: string;
    entityId: string;
    valueId: string;
    label: string;
    unit: string;
    value: Record<string, unknown>;
    confidence: number;
  }>;
};

export type LiveWorkbenchSnapshot = {
  generation: number;
  state: SimulatorWorkbenchState;
  measured: MeasuredTelemetryFrame[];
  twin: Omit<DigitalTwinState, "entities"> & {
    entities: Array<
      Omit<DigitalTwinState["entities"][number], "unitId" | "viewportEntity"> & {
        unitId?: string;
        viewportEntity?: TwinViewportEntity;
      }
    >;
  };
  lineage: WorkbenchLineage[];
  results: LiveSimopsResultFrame[];
};

export type AcceptedWorkbenchSnapshot = {
  generation: number;
  source: "live";
  input: WorkbenchProjectionInput;
};

export type WorkbenchSnapshotAdapter = {
  load(options?: { signal?: AbortSignal }): Promise<AcceptedWorkbenchSnapshot>;
};

export type WorkbenchReadErrorKind = "unavailable" | "empty" | "auth" | "schema" | "generation" | "partial";

export class WorkbenchReadError extends Error {
  constructor(
    readonly kind: WorkbenchReadErrorKind,
    message: string
  ) {
    super(message);
    this.name = "WorkbenchReadError";
  }
}

export type WorkbenchReadModel = {
  generation: number;
  source: "live" | "fixture";
  input: WorkbenchProjectionInput;
  acceptedAt: string;
};

export type WorkbenchReadState = {
  phase: "loading" | "live" | "fixture" | "stale" | "recovering" | "error";
  model: WorkbenchReadModel | null;
  message: string;
  errorKind?: WorkbenchReadErrorKind;
};

export type WorkbenchRefreshOptions = {
  allowFixtureFallback: boolean;
  fixtureInput: WorkbenchProjectionInput;
  now?: () => Date;
  signal?: AbortSignal;
};

export type WorkbenchRefreshCoordinator = {
  refresh(current: WorkbenchReadState): Promise<WorkbenchReadState | null>;
  dispose(): void;
};

export function initialWorkbenchReadState(): WorkbenchReadState {
  return { phase: "loading", model: null, message: "Loading one coherent live Workbench Snapshot." };
}

export function workbenchReadLabel(state: WorkbenchReadState): string {
  switch (state.phase) {
    case "live": return `Live generation ${state.model?.generation ?? "?"}`;
    case "fixture": return "Fixture fallback";
    case "stale": return `Stale live generation ${state.model?.generation ?? "?"}`;
    case "recovering": return "Recovering live Snapshot";
    case "error": return "Live Snapshot error";
    default: return "Loading live Snapshot";
  }
}

export function createHttpWorkbenchDataAdapter(
  fetcher: typeof fetch = fetch,
  base = (import.meta.env.VITE_SIMULATOR_WORKBENCH_API_BASE ?? "").replace(/\/$/, "")
): WorkbenchSnapshotAdapter {
  return {
    async load(options = {}) {
      let response: Response;
      try {
        response = await fetcher(`${base}/api/simulator-workbench/snapshot`, {
          method: "GET",
          credentials: "same-origin",
          ...(options.signal ? { signal: options.signal } : {}),
          headers: { Accept: "application/json" }
        });
      } catch (error) {
        throw new WorkbenchReadError("unavailable", error instanceof Error ? error.message : "Workbench unavailable");
      }
      if (response.status === 401 || response.status === 403) {
        throw new WorkbenchReadError("auth", `Workbench authorization failed (${response.status}).`);
      }
      if (response.status === 204 || response.status === 404) {
        throw new WorkbenchReadError("empty", `Workbench Snapshot is unavailable (${response.status}).`);
      }
      if (response.status === 502 || response.status === 503 || response.status === 504) {
        throw new WorkbenchReadError("unavailable", `Workbench service unavailable (${response.status}).`);
      }
      if (!response.ok) {
        throw new WorkbenchReadError("partial", `Workbench Snapshot request failed (${response.status}).`);
      }

      let snapshot: LiveWorkbenchSnapshot;
      try {
        snapshot = JSON.parse(await response.text()) as LiveWorkbenchSnapshot;
      } catch {
        throw new WorkbenchReadError("partial", "Workbench Snapshot response was truncated or invalid JSON.");
      }
      validateLiveWorkbenchSnapshot(snapshot);
      return { generation: snapshot.generation, source: "live" as const, input: projectLiveWorkbenchSnapshot(snapshot) };
    }
  };
}

export async function refreshWorkbenchReadState(
  current: WorkbenchReadState,
  adapter: WorkbenchSnapshotAdapter,
  options: WorkbenchRefreshOptions
): Promise<WorkbenchReadState> {
  const now = options.now ?? (() => new Date());
  try {
    const accepted = await adapter.load({ signal: options.signal });
    if (current.model?.source === "live" && accepted.generation < current.model.generation) {
      throw new WorkbenchReadError(
        "generation",
        `Workbench generation regressed from ${current.model.generation} to ${accepted.generation}.`
      );
    }
    return {
      phase: "live",
      model: { ...accepted, acceptedAt: now().toISOString() },
      message: `Live Workbench generation ${accepted.generation} accepted atomically.`
    };
  } catch (error) {
    const readError = normalizeReadError(error);
    if (current.model?.source === "live") {
      return {
        phase: "stale",
        model: current.model,
        message: `${readError.message} Retaining live generation ${current.model.generation} as stale.`,
        errorKind: readError.kind
      };
    }
    if (current.model?.source === "fixture") {
      return {
        phase: "fixture",
        model: current.model,
        message: `${readError.message} Retaining the explicit whole-Snapshot fixture fallback.`,
        errorKind: readError.kind
      };
    }
    if (options.allowFixtureFallback && (readError.kind === "unavailable" || readError.kind === "empty")) {
      return {
        phase: "fixture",
        model: { generation: 0, source: "fixture", input: options.fixtureInput, acceptedAt: now().toISOString() },
        message: `${readError.message} Using the explicit local-demo fixture Snapshot.`,
        errorKind: readError.kind
      };
    }
    return { phase: "error", model: null, message: readError.message, errorKind: readError.kind };
  }
}

export function createWorkbenchRefreshCoordinator(
  adapter: WorkbenchSnapshotAdapter,
  options: Omit<WorkbenchRefreshOptions, "signal">
): WorkbenchRefreshCoordinator {
  let active: AbortController | null = null;
  let disposed = false;
  return {
    async refresh(current) {
      active?.abort();
      const controller = new AbortController();
      active = controller;
      const next = await refreshWorkbenchReadState(current, adapter, { ...options, signal: controller.signal });
      if (disposed || active !== controller) return null;
      active = null;
      return next;
    },
    dispose() {
      disposed = true;
      active?.abort();
      active = null;
    }
  };
}

function validateLiveWorkbenchSnapshot(snapshot: LiveWorkbenchSnapshot): void {
  if (!snapshot || typeof snapshot !== "object" || !Number.isSafeInteger(snapshot.generation) || snapshot.generation < 0) {
    throw new WorkbenchReadError("schema", "Workbench Snapshot generation is invalid.");
  }
  if (!snapshot.state || snapshot.state.schemaVersion !== "simulator-workbench.state.v1") {
    throw new WorkbenchReadError("schema", "Workbench state schema is not supported.");
  }
  if (snapshot.state.snapshotGeneration !== snapshot.generation) {
    throw new WorkbenchReadError("generation", "Workbench state and Snapshot generation do not match.");
  }
  if (!Array.isArray(snapshot.measured) || !Array.isArray(snapshot.lineage) || !Array.isArray(snapshot.results) || !Array.isArray(snapshot.twin?.entities)) {
    throw new WorkbenchReadError("partial", "Workbench Snapshot omitted one or more read models.");
  }
  if (snapshot.twin.schemaVersion !== "digital-twin.state.v1") {
    throw new WorkbenchReadError("schema", "Workbench Twin State schema is not supported.");
  }
  if (snapshot.twin.entities.some((entity) => typeof entity.entityId !== "string" || !Array.isArray(entity.values))) {
    throw new WorkbenchReadError("partial", "Workbench Twin State contains an incomplete entity projection.");
  }
  if (snapshot.twin.entities.some((entity) => entity.values.some((value) => !isValueBasis(value.valueBasis)))) {
    throw new WorkbenchReadError("schema", "Workbench Twin State contains an invalid Value Basis.");
  }
  if (snapshot.measured.some((frame) => frame.schemaVersion !== "scada.telemetry.v1" || frame.valueBasis !== "measured")) {
    throw new WorkbenchReadError("schema", "Workbench Measured State schema or Value Basis is invalid.");
  }
  if (snapshot.results.some((frame) => frame.schemaVersion !== "simops.result.v1" || frame.valueBasis !== "simulated")) {
    throw new WorkbenchReadError("schema", "Workbench Simulated Result State schema or Value Basis is invalid.");
  }
  if (snapshot.lineage.some((record) => record.schemaVersion !== "digital-twin.lineage.v1" || !isValueBasis(record.valueBasis))) {
    throw new WorkbenchReadError("schema", "Workbench Lineage schema is not supported.");
  }
  const isEmpty = snapshot.measured.length === 0 && snapshot.results.length === 0 && snapshot.twin.entities.length === 0 && snapshot.lineage.length === 0;
  if (isEmpty) {
    throw new WorkbenchReadError("empty", "Workbench store has no coherent read models yet.");
  }
}

function isValueBasis(value: unknown): value is WorkbenchValueBasis {
  return value === "measured" || value === "imputed" || value === "simulated";
}

function projectLiveWorkbenchSnapshot(snapshot: LiveWorkbenchSnapshot): WorkbenchProjectionInput {
  const reactorIDs = new Set(snapshot.measured.map((frame) => frame.reactorId?.trim()).filter(isNonEmptyString));
  const entities: DigitalTwinState["entities"] = snapshot.twin.entities.flatMap((entity) => {
    const unitId = reactorIDForEntity(entity, snapshot.measured, reactorIDs);
    return unitId ? [{
      ...entity,
      unitId,
      viewportEntity: entity.viewportEntity || inferViewportEntity(entity.displayName, entity.values)
    }] : [];
  });
  appendMissingMeasuredValues(entities, snapshot.measured);
  appendMissingResultValues(entities, snapshot.results, snapshot.measured, reactorIDs);
  if (entities.length === 0) {
    throw new WorkbenchReadError("partial", "Workbench Snapshot cannot identify a live projection entity.");
  }
  const unitIds = [...new Set(entities.map((entity) => entity.unitId))];
  const fleetUnits = unitIds.map((unitId, index) => liveFleetUnit(unitId, index, snapshot.state.generatedAt));
  return {
    state: {
      ...snapshot.state,
      selectedUnitId: unitIds[0],
      fleetUnitRefs: unitIds,
      commercialDisplayBasisRefs: []
    },
    measured: snapshot.measured,
    twin: { ...snapshot.twin, entities },
    lineages: snapshot.lineage,
    fleetUnits,
    commercialDisplayBasis: []
  };
}

function appendMissingMeasuredValues(
  entities: DigitalTwinState["entities"],
  frames: MeasuredTelemetryFrame[]
): void {
  for (const frame of frames) {
    const unitId = frame.reactorId?.trim();
    if (!unitId) continue;
    const entity = ensureLiveEntity(entities, unitId);
    const valueId = `measured:${frame.tagId}`;
    if (entity.values.some((value) =>
      value.valueId === valueId || (value.valueBasis === "measured" && value.sourceIds.includes(frame.tagId))
    )) continue;
    entity.values.push({
      valueId,
      label: frame.tagId,
      valueBasis: "measured",
      unit: frame.unit,
      value: frame.value,
      confidence: frame.quality === "good" ? 1 : 0.5,
      freshness: { ageSec: 0, status: frame.quality === "stale" ? "stale" : "unknown" },
      lineageId: `lineage:${frame.tagId}`,
      sourceIds: [frame.tagId]
    });
  }
}

function appendMissingResultValues(
  entities: DigitalTwinState["entities"],
  results: LiveSimopsResultFrame[],
  measured: MeasuredTelemetryFrame[],
  reactorIDs: Set<string>
): void {
  for (const frame of results) {
    for (const value of frame.values) {
      const unitId = reactorIDs.has(value.entityId)
        ? value.entityId
        : measured.find((measuredFrame) => measuredFrame.assetId === value.entityId)?.reactorId?.trim();
      if (!unitId) continue;
      const entity = ensureLiveEntity(entities, unitId);
      if (entity.values.some((candidate) => candidate.valueId === value.valueId)) continue;
      entity.values.push({
        valueId: value.valueId,
        label: value.label,
        valueBasis: "simulated",
        unit: value.unit,
        value: value.value,
        confidence: value.confidence,
        freshness: { ageSec: 0, status: "unknown" },
        lineageId: `lineage:${value.valueId}`,
        sourceIds: [frame.runId]
      });
    }
  }
}

function ensureLiveEntity(
  entities: DigitalTwinState["entities"],
  unitId: string
) {
  const existing = entities.find((entity) => entity.unitId === unitId || entity.entityId === unitId);
  if (existing) return existing;
  const created = { entityId: unitId, unitId, displayName: unitId, viewportEntity: "core" as const, values: [] as WorkbenchValue[] };
  entities.push(created);
  return created;
}

function reactorIDForEntity(
  entity: LiveWorkbenchSnapshot["twin"]["entities"][number],
  measured: MeasuredTelemetryFrame[],
  reactorIDs: Set<string>
): string | null {
  if (reactorIDs.has(entity.entityId)) return entity.entityId;
  const sourceIDs = new Set(entity.values.flatMap((value) => value.sourceIds));
  return measured.find((frame) =>
    Boolean(frame.reactorId?.trim()) && (frame.assetId === entity.entityId || sourceIDs.has(frame.tagId))
  )?.reactorId?.trim() || null;
}

function isNonEmptyString(value: string | undefined): value is string {
  return Boolean(value);
}

function inferViewportEntity(displayName: string, values: WorkbenchValue[]): TwinViewportEntity {
  const text = `${displayName} ${values.map((value) => value.label).join(" ")}`.toLowerCase();
  if (text.includes("circulator")) return "circulators";
  if (text.includes("heat exchanger")) return "heatExchangers";
  if (text.includes("vessel")) return "vessel";
  if (text.includes("control drum")) return "controlDrums";
  return "core";
}

function liveFleetUnit(unitId: string, index: number, generatedAt: string): KaleidosUnitSummary {
  return {
    unitId,
    displayName: unitId,
    availabilityPhase: "status not provided",
    commercialMode: "commercial basis not provided",
    breakerToBreakerLabel: "output not provided",
    electricOutputMwe: 0,
    usefulThermalOutputMwt: 0,
    residualHeatMwth: null,
    accruedDisplayValue: { compactLabel: "commercial display basis not provided", amountKUsd: null, estimate: false },
    freshness: { status: "fresh", ageSec: 0 },
    commercialBasisId: `live:${index}:${unitId}`,
    emphasis: `Live Workbench Snapshot accepted at ${generatedAt}`
  };
}

function normalizeReadError(error: unknown): WorkbenchReadError {
  return error instanceof WorkbenchReadError
    ? error
    : new WorkbenchReadError("partial", error instanceof Error ? error.message : "Workbench refresh failed.");
}
