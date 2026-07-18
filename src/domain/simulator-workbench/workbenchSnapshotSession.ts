import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import {
  buildWorkbenchProjection,
  type WorkbenchProjection,
  type WorkbenchProjectionInput,
  type WorkbenchSelection
} from "./projection";
import {
  createHttpWorkbenchDataAdapter,
  WorkbenchReadError,
  type WorkbenchSnapshotAdapter
} from "./liveWorkbench";
import { projectHealthCards } from "./workbenchHealthPanelProjection";
import type { SimulationHealthPanelModel } from "../../components/simulator-workbench/SimulationHealthPanel";

export type WorkbenchReadModel = {
  generation: number;
  source: "live" | "fixture";
  input: WorkbenchProjectionInput;
  healthPanelModel: SimulationHealthPanelModel;
  acceptedAt: string;
};

export type WorkbenchReadState = {
  phase: "loading" | "live" | "fixture" | "stale" | "recovering" | "error";
  model: WorkbenchReadModel | null;
  message: string;
  errorKind?: WorkbenchReadError["kind"];
};

type WorkbenchRefreshOptions = {
  allowFixtureFallback: boolean;
  fixtureInput: WorkbenchProjectionInput;
  now?: () => Date;
  signal?: AbortSignal;
};

function initialWorkbenchReadState(): WorkbenchReadState {
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

export type WorkbenchSnapshotSession = {
  getResult(): WorkbenchSnapshotSessionResult;
  subscribe(listener: (result: WorkbenchSnapshotSessionResult) => void): () => void;
  start(): Promise<void>;
  refresh(): Promise<void>;
  dispose(): void;
};

export type WorkbenchSnapshotSessionResult = {
  readState: WorkbenchReadState;
  projection: WorkbenchProjection | null;
  selection: WorkbenchSelection;
  refresh(): Promise<void>;
  selectUnit(unitId: string, commercialBasisId?: string): void;
  selectValue(valueId: string): void;
  selectCommercialBasis(commercialBasisId?: string): void;
};

export type WorkbenchSnapshotSessionOptions = Omit<WorkbenchRefreshOptions, "signal"> & {
  initialSelection?: WorkbenchSelection;
  refreshIntervalMs: number;
};

const DEFAULT_REFRESH_INTERVAL_MS = 10_000;

export function createBrowserWorkbenchSnapshotSession(
  initialSelection: WorkbenchSelection = {}
): WorkbenchSnapshotSession {
  return createWorkbenchSnapshotSession(createHttpWorkbenchDataAdapter(), {
    allowFixtureFallback: import.meta.env.VITE_WORKBENCH_ALLOW_FIXTURE_FALLBACK === "true",
    fixtureInput: loadFixtureWorkbenchData(),
    initialSelection,
    refreshIntervalMs: DEFAULT_REFRESH_INTERVAL_MS
  });
}

export function createWorkbenchSnapshotSession(
  adapter: WorkbenchSnapshotAdapter,
  options: WorkbenchSnapshotSessionOptions
): WorkbenchSnapshotSession {
  let state = initialWorkbenchReadState();
  let settledState = state;
  let active: AbortController | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let running = false;
  let hasSettledRead = false;
  let selection = options.initialSelection ?? {};
  let projection: WorkbenchProjection | null = null;
  let result: WorkbenchSnapshotSessionResult;
  const listeners = new Set<(result: WorkbenchSnapshotSessionResult) => void>();

  function updateResult(): void {
    result = {
      readState: state,
      projection,
      selection,
      refresh,
      selectUnit,
      selectValue,
      selectCommercialBasis
    };
  }

  function publish(next: WorkbenchReadState): void {
    state = next;
    updateResult();
    listeners.forEach((listener) => listener(result));
  }

  function settle(next: WorkbenchReadState): void {
    if (next.model) {
      projection = buildWorkbenchProjection(next.model.input, selection);
      selection = reconcileSelection(next.model.input, selection, projection);
    }
    settledState = next;
    publish(next);
  }

  function applySelection(next: WorkbenchSelection): void {
    selection = next;
    if (state.model) {
      projection = buildWorkbenchProjection(state.model.input, selection);
      selection = reconcileSelection(state.model.input, selection, projection);
    }
    publish(state);
  }

  function selectUnit(unitId: string, commercialBasisId?: string): void {
    applySelection({
      selectedUnitId: unitId,
      ...(commercialBasisId ? { selectedCommercialBasisId: commercialBasisId } : {})
    });
  }

  function selectValue(valueId: string): void {
    applySelection({ ...selection, selectedValueId: valueId });
  }

  function selectCommercialBasis(commercialBasisId?: string): void {
    const next = { ...selection };
    if (commercialBasisId) next.selectedCommercialBasisId = commercialBasisId;
    else delete next.selectedCommercialBasisId;
    applySelection(next);
  }

  function getResult(): WorkbenchSnapshotSessionResult {
    return result;
  }

  function clearScheduledRefresh(): void {
    if (timer !== null) clearTimeout(timer);
    timer = null;
  }

  function scheduleRefresh(): void {
    clearScheduledRefresh();
    timer = setTimeout(() => {
      timer = null;
      void refresh();
    }, options.refreshIntervalMs);
  }

  async function refresh(): Promise<void> {
    if (!running) return;
    clearScheduledRefresh();
    active?.abort();
    const controller = new AbortController();
    const acceptedState = settledState;
    active = controller;

    const pending: WorkbenchReadState = {
      ...acceptedState,
      phase: acceptedState.model ? "recovering" : "loading",
      message: acceptedState.model
        ? "Refreshing one coherent live Workbench Snapshot."
        : acceptedState.message
    };
    publish(pending);

    const next = await refreshWorkbenchReadState(acceptedState, adapter, {
      ...options,
      allowFixtureFallback: options.allowFixtureFallback && !hasSettledRead,
      signal: controller.signal
    });
    if (!running || active !== controller) return;
    active = null;
    hasSettledRead = true;
    settle(next);
    scheduleRefresh();
  }

  updateResult();

  return {
    getResult,
    subscribe(listener) {
      listeners.add(listener);
      listener(result);
      return () => listeners.delete(listener);
    },
    async start() {
      if (running) return;
      running = true;
      await refresh();
    },
    refresh,
    dispose() {
      running = false;
      clearScheduledRefresh();
      active?.abort();
      active = null;
      state = settledState;
      updateResult();
    }
  };
}

function reconcileSelection(
  input: WorkbenchProjectionInput,
  requested: WorkbenchSelection,
  projection: WorkbenchProjection
): WorkbenchSelection {
  const selectedUnitId = projection.selectedUnit.unitId;
  const selectedValueId = projection.selectedValue?.valueId;
  const commercialBasisIsValid = requested.selectedCommercialBasisId
    ? input.commercialDisplayBasis.some(
        (basis) => basis.basisId === requested.selectedCommercialBasisId && basis.unitId === selectedUnitId
      )
    : false;

  return {
    selectedUnitId,
    ...(selectedValueId ? { selectedValueId } : {}),
    ...(commercialBasisIsValid ? { selectedCommercialBasisId: requested.selectedCommercialBasisId } : {})
  };
}

async function refreshWorkbenchReadState(
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
    const acceptedAt = now();
    return {
      phase: "live",
      model: {
        ...accepted,
        healthPanelModel: healthFromSnapshot(accepted.input, acceptedAt),
        acceptedAt: acceptedAt.toISOString()
      },
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
    if (
      current.model?.source === "fixture" &&
      (readError.kind === "unavailable" || readError.kind === "empty")
    ) {
      return {
        phase: "fixture",
        model: current.model,
        message: `${readError.message} Retaining the explicit whole-Snapshot fixture fallback.`,
        errorKind: readError.kind
      };
    }
    if (current.model?.source === "fixture") {
      return {
        phase: "error",
        model: current.model,
        message: `${readError.message} The existing fixture remains visible but did not satisfy this live read.`,
        errorKind: readError.kind
      };
    }
    if (options.allowFixtureFallback && (readError.kind === "unavailable" || readError.kind === "empty")) {
      const acceptedAt = now();
      return {
        phase: "fixture",
        model: {
          generation: 0,
          source: "fixture",
          input: options.fixtureInput,
          healthPanelModel: healthFromSnapshot(options.fixtureInput, acceptedAt),
          acceptedAt: acceptedAt.toISOString()
        },
        message: `${readError.message} Using the explicit local-demo fixture Snapshot.`,
        errorKind: readError.kind
      };
    }
    return { phase: "error", model: null, message: readError.message, errorKind: readError.kind };
  }
}

function healthFromSnapshot(input: WorkbenchProjectionInput, observedAt: Date): SimulationHealthPanelModel {
  return projectHealthCards({
    generatedAt: input.state.generatedAt,
    activeSimulationRuns: input.state.activeSimulationRuns
  }, observedAt);
}

function normalizeReadError(error: unknown): WorkbenchReadError {
  return error instanceof WorkbenchReadError
    ? error
    : new WorkbenchReadError("partial", error instanceof Error ? error.message : "Workbench refresh failed.");
}
