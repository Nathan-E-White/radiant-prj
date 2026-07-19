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
  scheduler?: WorkbenchSnapshotScheduler;
};

export type WorkbenchSnapshotScheduler = {
  schedule(task: () => void, delayMs: number): unknown;
  cancel(handle: unknown): void;
};

const DEFAULT_REFRESH_INTERVAL_MS = 10_000;
const browserScheduler: WorkbenchSnapshotScheduler = {
  schedule: (task, delayMs) => setTimeout(task, delayMs),
  cancel: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>)
};

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
  const scheduler = options.scheduler ?? browserScheduler;
  const configuration: WorkbenchSnapshotSessionOptions = {
    allowFixtureFallback: options.allowFixtureFallback,
    fixtureInput: structuredClone(options.fixtureInput),
    refreshIntervalMs: options.refreshIntervalMs,
    ...(options.now ? { now: options.now } : {})
  };
  let state = immutableSnapshot(initialWorkbenchReadState());
  let settledState = state;
  let active: AbortController | null = null;
  let timer: unknown | null = null;
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
    state = immutableSnapshot(next);
    listeners.forEach((listener) => listener(state));
  }

  function settle(next: WorkbenchReadState): void {
    state = immutableSnapshot(next);
    settledState = state;
    listeners.forEach((listener) => listener(state));
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
    if (timer !== null) scheduler.cancel(timer);
    timer = null;
  }

  function scheduleRefresh(): void {
    clearScheduledRefresh();
    timer = scheduler.schedule(() => {
      timer = null;
      void refresh();
    }, configuration.refreshIntervalMs);
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
      ...configuration,
      allowFixtureFallback: configuration.allowFixtureFallback && !hasSettledRead,
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

function immutableSnapshot(state: WorkbenchReadState): WorkbenchReadState {
  const snapshot = structuredClone(state);
  if (state.model && Object.isFrozen(state.model)) snapshot.model = state.model;
  return deepFreeze(snapshot);
}

function deepFreeze<T>(value: T): T {
  if (value && typeof value === "object" && !Object.isFrozen(value)) {
    Object.freeze(value);
    Object.values(value).forEach(deepFreeze);
  }
  return value;
}
