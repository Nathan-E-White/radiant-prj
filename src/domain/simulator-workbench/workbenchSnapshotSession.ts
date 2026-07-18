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
};

const DEFAULT_REFRESH_INTERVAL_MS = 10_000;

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
  let state = initialWorkbenchReadState();
  let settledState = state;
  let active: AbortController | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let running = false;
  let hasSettledRead = false;
  const listeners = new Set<(state: WorkbenchReadState) => void>();

  function publish(next: WorkbenchReadState): void {
    state = next;
    listeners.forEach((listener) => listener(state));
  }

  function settle(next: WorkbenchReadState): void {
    settledState = next;
    publish(next);
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
