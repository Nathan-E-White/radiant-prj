import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import {
  createHttpWorkbenchDataAdapter,
  initialWorkbenchReadState,
  refreshWorkbenchReadState,
  type WorkbenchReadState,
  type WorkbenchRefreshOptions,
  type WorkbenchSnapshotAdapter
} from "./liveWorkbench";

export type WorkbenchSnapshotSession = {
  getState(): WorkbenchReadState;
  subscribe(listener: (state: WorkbenchReadState) => void): () => void;
  start(): Promise<void>;
  refresh(): Promise<void>;
  dispose(): void;
};

export type WorkbenchSnapshotSessionOptions = Omit<WorkbenchRefreshOptions, "signal"> & {
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

export function createBrowserWorkbenchSnapshotSession(): WorkbenchSnapshotSession {
  return createWorkbenchSnapshotSession(createHttpWorkbenchDataAdapter(), {
    allowFixtureFallback: import.meta.env.VITE_WORKBENCH_ALLOW_FIXTURE_FALLBACK === "true",
    fixtureInput: loadFixtureWorkbenchData(),
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
  const listeners = new Set<(state: WorkbenchReadState) => void>();

  function publish(next: WorkbenchReadState): void {
    state = immutableSnapshot(next);
    listeners.forEach((listener) => listener(state));
  }

  function settle(next: WorkbenchReadState): void {
    state = immutableSnapshot(next);
    settledState = state;
    listeners.forEach((listener) => listener(state));
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
    dispose() {
      running = false;
      clearScheduledRefresh();
      active?.abort();
      active = null;
      state = settledState;
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
