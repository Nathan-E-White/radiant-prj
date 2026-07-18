import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import {
  buildWorkbenchProjection,
  type WorkbenchProjection,
  type WorkbenchProjectionInput,
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
