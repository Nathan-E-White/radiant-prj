import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import {
  buildWorkbenchProjectionResult,
  type WorkbenchProjection,
  type WorkbenchSelection
} from "./projection";
import {
  createHttpWorkbenchDataAdapter,
  initialWorkbenchReadState,
  refreshWorkbenchReadState,
  type WorkbenchReadState,
  type WorkbenchRefreshOptions,
  type WorkbenchSnapshotAdapter
} from "./liveWorkbench";

export type WorkbenchSnapshotSession = {
  getState(): WorkbenchSnapshotResult;
  subscribe(listener: (state: WorkbenchSnapshotResult) => void): () => void;
  start(): Promise<void>;
  refresh(): Promise<void>;
  selectUnit(unitId: string): void;
  selectValue(valueId: string): void;
  dispose(): void;
};

export type WorkbenchSnapshotResult = {
  readState: WorkbenchReadState;
  projection: WorkbenchProjection | null;
  selection: WorkbenchSelection;
};

export type WorkbenchSnapshotSessionOptions = Omit<WorkbenchRefreshOptions, "signal"> & {
  refreshIntervalMs: number;
  scheduler?: WorkbenchSnapshotScheduler;
  initialSelection?: WorkbenchSelection;
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

export function createBrowserWorkbenchSnapshotSession(initialSelection: WorkbenchSelection = {}): WorkbenchSnapshotSession {
  return createWorkbenchSnapshotSession(createHttpWorkbenchDataAdapter(), {
    allowFixtureFallback: import.meta.env.VITE_WORKBENCH_ALLOW_FIXTURE_FALLBACK === "true",
    fixtureInput: loadFixtureWorkbenchData(),
    refreshIntervalMs: DEFAULT_REFRESH_INTERVAL_MS,
    initialSelection
  });
}

export function createWorkbenchSnapshotSession(
  adapter: WorkbenchSnapshotAdapter,
  options: WorkbenchSnapshotSessionOptions
): WorkbenchSnapshotSession {
  const scheduler = options.scheduler ?? browserScheduler;
  let requestedSelection = structuredClone(options.initialSelection ?? {});
  const configuration: WorkbenchSnapshotSessionOptions = {
    allowFixtureFallback: options.allowFixtureFallback,
    fixtureInput: structuredClone(options.fixtureInput),
    refreshIntervalMs: options.refreshIntervalMs,
    ...(options.now ? { now: options.now } : {})
  };
  let settledReadState = initialWorkbenchReadState();
  let state = immutableResult(projectResult(settledReadState, requestedSelection));
  let active: AbortController | null = null;
  let timer: unknown | null = null;
  let running = false;
  let hasSettledRead = false;
  const listeners = new Set<(state: WorkbenchSnapshotResult) => void>();

  function publish(next: WorkbenchReadState): void {
    state = immutableResult(projectResult(next, requestedSelection));
    listeners.forEach((listener) => listener(state));
  }

  function settle(next: WorkbenchReadState): void {
    publish(next);
    settledReadState = state.readState;
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
    const acceptedState = settledReadState;
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

  return {
    getState() {
      return state;
    },
    subscribe(listener) {
      listeners.add(listener);
      listener(state);
      return () => listeners.delete(listener);
    },
    async start() {
      if (running) return;
      running = true;
      await refresh();
    },
    refresh,
    selectUnit(unitId) {
      if (state.selection.selectedUnitId === unitId) return;
      const selectedUnit = state.projection?.fleetUnits.find((unit) => unit.unitId === unitId);
      const input = state.readState.model?.input;
      if (!selectedUnit || !input) return;
      const commercialBasis = input.commercialDisplayBasis.find(
        (basis) => basis.unitId === unitId && basis.basisId === selectedUnit.commercialBasisId
      );
      requestedSelection = {
        selectedUnitId: unitId,
        ...(commercialBasis ? { selectedCommercialBasisId: commercialBasis.basisId } : {})
      };
      publish(state.readState);
    },
    selectValue(valueId) {
      const visibleValue = state.projection
        ? Object.values(state.projection.groups).flatMap((group) => group.values).find((value) => value.valueId === valueId)
        : undefined;
      if (!visibleValue || state.selection.selectedValueId === valueId) return;
      requestedSelection = { ...requestedSelection, selectedValueId: valueId };
      publish(state.readState);
    },
    dispose() {
      running = false;
      clearScheduledRefresh();
      active?.abort();
      active = null;
      state = immutableResult(projectResult(settledReadState, requestedSelection));
    }
  };
}

function projectResult(readState: WorkbenchReadState, selection: WorkbenchSelection): WorkbenchSnapshotResult {
  if (!readState.model) return { readState, projection: null, selection };
  const projected = buildWorkbenchProjectionResult(readState.model.input, selection);
  return { readState, ...projected };
}

function immutableResult(result: WorkbenchSnapshotResult): WorkbenchSnapshotResult {
  const snapshot = structuredClone(result);
  if (result.readState.model && Object.isFrozen(result.readState.model)) snapshot.readState.model = result.readState.model;
  return deepFreeze(snapshot);
}

function deepFreeze<T>(value: T): T {
  if (value && typeof value === "object" && !Object.isFrozen(value)) {
    Object.freeze(value);
    Object.values(value).forEach(deepFreeze);
  }
  return value;
}
