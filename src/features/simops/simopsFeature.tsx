import { RefreshCcw, Play, Square } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  createSimopsRun as createSimopsRunRequest,
  getSimopsRun as getSimopsRunRequest,
  listSimopsEvents as listSimopsEventsRequest,
  stopSimopsRun as stopSimopsRunRequest,
  type SimopsEvent,
  type SimopsEventsResponse,
  type SimopsLifecycle,
  type SimopsRunRequest,
  type SimopsRunResponse,
  type SimopsWorkerKind
} from "../../api/simops";
import { StatusPill } from "../shared/presentation";

export const simopsScenarios = ["nominal", "scheduler-drift", "checkpoint-pressure", "cloud-burst", "fabric-warning"];
export const simopsWorkerOptions: SimopsWorkerKind[] = ["scheduler", "storage", "burst", "fabric"];
export const simopsLaunchModes: Array<SimopsRunRequest["launch_mode"]> = ["resident", "spawn", "auto"];

export type SimOpsClient = {
  createSimopsRun: (payload: SimopsRunRequest) => Promise<SimopsRunResponse>;
  getSimopsRun: (runId: string) => Promise<SimopsRunResponse>;
  stopSimopsRun: (runId: string) => Promise<SimopsRunResponse>;
  listSimopsEvents: (runId: string) => Promise<SimopsEventsResponse>;
};

export type SimOpsValidation = {
  valid: boolean;
  message?: string;
};

export function isSimopsRunActive(lifecycle: SimopsLifecycle): boolean {
  return lifecycle === "created" || lifecycle === "starting" || lifecycle === "streaming" || lifecycle === "degraded";
}

export function isSimopsRunTerminal(lifecycle: SimopsLifecycle): boolean {
  return lifecycle === "complete" || lifecycle === "failed" || lifecycle === "stopped";
}

export function toggleSimopsWorker(current: readonly SimopsWorkerKind[], nextKind: SimopsWorkerKind): SimopsWorkerKind[] {
  if (current.includes(nextKind)) {
    return current.filter((kind) => kind !== nextKind);
  }
  return [...current, nextKind];
}

export function buildSimopsRunRequest(input: {
  scenario: string;
  source: string;
  launchMode: SimopsRunRequest["launch_mode"];
  workerKinds: SimopsWorkerKind[];
  runtimeLimit: number;
  idempotencyKey?: string;
}): SimopsRunRequest {
  const payload: SimopsRunRequest = {
    scenario_id: input.scenario,
    source: input.source,
    work_script: input.scenario,
    launch_mode: input.launchMode,
    worker_kinds: input.workerKinds,
    runtime_limit_sec: input.runtimeLimit
  };

  if (input.idempotencyKey?.trim()) {
    payload.idempotency_key = input.idempotencyKey.trim();
  }

  return payload;
}

export function validateSimopsRun(input: {
  workerKinds: SimopsWorkerKind[];
  runtimeLimit: number;
}): SimOpsValidation {
  if (input.workerKinds.length === 0) {
    return {
      valid: false,
      message: "Choose at least one worker kind."
    };
  }

  if (input.runtimeLimit < 1 || input.runtimeLimit > 3600) {
    return {
      valid: false,
      message: "Runtime limit must stay between 1 and 3600 seconds."
    };
  }

  return { valid: true };
}

const simopsClient: SimOpsClient = {
  createSimopsRun: createSimopsRunRequest,
  getSimopsRun: getSimopsRunRequest,
  stopSimopsRun: stopSimopsRunRequest,
  listSimopsEvents: listSimopsEventsRequest
};

type SimOpsController = {
  message: string;
  error: string;
  scenario: string;
  source: string;
  launchMode: SimopsRunRequest["launch_mode"];
  runtimeLimit: number;
  workerKinds: SimopsWorkerKind[];
  idempotencyKey: string;
  activeRun: SimopsRunResponse | null;
  events: SimopsEvent[];
  submitting: boolean;
  setScenario: (scenario: string) => void;
  setSource: (source: string) => void;
  setLaunchMode: (mode: SimopsRunRequest["launch_mode"]) => void;
  setRuntimeLimit: (seconds: number) => void;
  setIdempotencyKey: (key: string) => void;
  toggleWorker: (kind: SimopsWorkerKind) => void;
  createRun: () => Promise<void>;
  stopRun: () => Promise<void>;
  refresh: () => void;
  isRunActive: boolean;
  isRunComplete: boolean;
};

export function useSimOpsController(config: { client?: SimOpsClient } = {}): SimOpsController {
  const { client = simopsClient } = config;
  const [simopsScenario, setSimopsScenario] = useState(simopsScenarios[0]);
  const [simopsSource, setSimopsSource] = useState("frontend");
  const [simopsLaunchMode, setSimopsLaunchMode] = useState<SimopsRunRequest["launch_mode"]>("auto");
  const [simopsRuntimeLimit, setSimopsRuntimeLimit] = useState(120);
  const [simopsWorkerKinds, setSimopsWorkerKinds] = useState<SimopsWorkerKind[]>(["scheduler", "storage"]);
  const [simopsIdempotency, setSimopsIdempotency] = useState("");
  const [simopsRun, setSimopsRun] = useState<SimopsRunResponse | null>(null);
  const [simopsEvents, setSimopsEvents] = useState<SimopsEvent[]>([]);
  const [simopsMessage, setSimopsMessage] = useState("No run selected.");
  const [simopsError, setSimopsError] = useState("");
  const [isSubmittingSimops, setIsSubmittingSimops] = useState(false);

  const refreshEvents = useCallback(
    async (runId: string) => {
      const response = await client.listSimopsEvents(runId);
      setSimopsEvents(response.events);
    },
    [client]
  );

  const refreshSimopsRun = useCallback(
    async (runId: string) => {
      if (!runId) {
        return;
      }
      const next = await client.getSimopsRun(runId);
      setSimopsRun(next);
      await refreshEvents(runId);
      setSimopsError("");
    },
    [client, refreshEvents]
  );

  const createSimopsRun = useCallback(async () => {
    const validation = validateSimopsRun({ workerKinds: simopsWorkerKinds, runtimeLimit: simopsRuntimeLimit });
    if (!validation.valid) {
      setSimopsError(validation.message ?? "Invalid payload");
      return;
    }

    setIsSubmittingSimops(true);
    setSimopsError("");
    setSimopsMessage("Launching simops run...");

    const payload = buildSimopsRunRequest({
      scenario: simopsScenario,
      source: simopsSource,
      launchMode: simopsLaunchMode,
      workerKinds: simopsWorkerKinds,
      runtimeLimit: simopsRuntimeLimit,
      idempotencyKey: simopsIdempotency
    });

    try {
      const run = await client.createSimopsRun(payload);
      setSimopsRun(run);
      setSimopsMessage("Run launched. Polling for worker status...");
      await refreshEvents(run.run_id);
      void refreshSimopsRun(run.run_id);
    } catch (error) {
      setSimopsError(`simops create failed: ${(error as Error).message}`);
    } finally {
      setIsSubmittingSimops(false);
    }
  }, [simopsScenario, simopsSource, simopsLaunchMode, simopsWorkerKinds, simopsRuntimeLimit, simopsIdempotency, client, refreshEvents, refreshSimopsRun]);

  const stopSimopsRun = useCallback(async () => {
    if (!simopsRun?.run_id) {
      return;
    }

    setIsSubmittingSimops(true);
    setSimopsError("");
    setSimopsMessage("Issuing stop command...");
    try {
      const run = await client.stopSimopsRun(simopsRun.run_id);
      setSimopsRun(run);
      await refreshEvents(run.run_id);
      setSimopsMessage("Stop requested for run.");
    } catch (error) {
      setSimopsError(`simops stop failed: ${(error as Error).message}`);
    } finally {
      setIsSubmittingSimops(false);
    }
  }, [simopsRun?.run_id, client, refreshEvents]);

  const refresh = useCallback(() => {
    if (simopsRun) {
      void refreshSimopsRun(simopsRun.run_id);
    } else {
      setSimopsError("No run selected to refresh.");
    }
  }, [simopsRun, refreshSimopsRun]);

  const isRunActive = simopsRun !== null && isSimopsRunActive(simopsRun.lifecycle);
  const isRunComplete = simopsRun !== null && isSimopsRunTerminal(simopsRun.lifecycle);

  const toggleWorkerKind = useCallback((kind: SimopsWorkerKind) => {
    setSimopsWorkerKinds((previous) => toggleSimopsWorker(previous, kind));
  }, []);

  useEffect(() => {
    if (!isRunActive || !simopsRun) {
      return;
    }

    const poll = window.setInterval(() => {
      void refreshSimopsRun(simopsRun.run_id);
    }, 2500);

    return () => {
      window.clearInterval(poll);
    };
  }, [isRunActive, simopsRun?.run_id, refreshSimopsRun]);

  return useMemo(
    () => ({
      message: simopsMessage,
      error: simopsError,
      scenario: simopsScenario,
      source: simopsSource,
      launchMode: simopsLaunchMode,
      runtimeLimit: simopsRuntimeLimit,
      workerKinds: simopsWorkerKinds,
      idempotencyKey: simopsIdempotency,
      activeRun: simopsRun,
      events: simopsEvents,
      submitting: isSubmittingSimops,
      setScenario: setSimopsScenario,
      setSource: setSimopsSource,
      setLaunchMode: setSimopsLaunchMode,
      setRuntimeLimit: setSimopsRuntimeLimit,
      setIdempotencyKey: setSimopsIdempotency,
      toggleWorker: toggleWorkerKind,
      createRun: createSimopsRun,
      stopRun: stopSimopsRun,
      refresh,
      isRunActive,
      isRunComplete
    }),
    [
      simopsMessage,
      simopsError,
      simopsScenario,
      simopsSource,
      simopsLaunchMode,
      simopsRuntimeLimit,
      simopsWorkerKinds,
      simopsIdempotency,
      simopsRun,
      simopsEvents,
      isSubmittingSimops,
      isRunActive,
      isRunComplete,
      toggleWorkerKind,
      createSimopsRun,
      stopSimopsRun,
      refresh
    ]
  );
}

export function SimOpsControlPanel({
  message,
  error,
  scenario,
  source,
  launchMode,
  runtimeLimit,
  workerKinds,
  idempotencyKey,
  activeRun,
  events,
  submitting,
  onScenarioChange,
  onSourceChange,
  onLaunchModeChange,
  onRuntimeLimitChange,
  onIdempotencyChange,
  onWorkerToggle,
  onCreate,
  onStop,
  onRefresh
}: {
  message: string;
  error: string;
  scenario: string;
  source: string;
  launchMode: SimopsRunRequest["launch_mode"];
  runtimeLimit: number;
  workerKinds: SimopsWorkerKind[];
  idempotencyKey: string;
  activeRun: SimopsRunResponse | null;
  events: SimopsEvent[];
  submitting: boolean;
  onScenarioChange: (scenario: string) => void;
  onSourceChange: (source: string) => void;
  onLaunchModeChange: (mode: SimopsRunRequest["launch_mode"]) => void;
  onRuntimeLimitChange: (seconds: number) => void;
  onIdempotencyChange: (next: string) => void;
  onWorkerToggle: (kind: SimopsWorkerKind) => void;
  onCreate: () => Promise<void>;
  onStop: () => Promise<void>;
  onRefresh: () => void;
}) {
  const isRunActive = activeRun !== null && isSimopsRunActive(activeRun.lifecycle);
  const isRunComplete = activeRun !== null && isSimopsRunTerminal(activeRun.lifecycle);

  return (
    <section className="content-grid simops-grid">
      <div className="panel simops-control">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">SimOps control-plane</p>
            <h2>Containerized worker orchestration</h2>
          </div>
          <button className="icon-command" onClick={onRefresh} type="button" title="Refresh run">
            <RefreshCcw size={18} />
          </button>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-scenario">Scenario</label>
          <select id="simops-scenario" value={scenario} onChange={(event) => onScenarioChange(event.target.value)}>
            {simopsScenarios.map((option) => (
              <option value={option} key={option}>
                {option}
              </option>
            ))}
          </select>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-source">Source</label>
          <select id="simops-source" value={source} onChange={(event) => onSourceChange(event.target.value)}>
            <option value="frontend">frontend</option>
            <option value="work-script">work-script</option>
          </select>
        </div>
        <div className="simops-field">
          <label>Workers</label>
          <div className="simops-checkbox-group">
            {simopsWorkerOptions.map((worker) => (
              <label className="simops-checkbox" key={worker}>
                <input checked={workerKinds.includes(worker)} onChange={() => onWorkerToggle(worker)} type="checkbox" />
                {worker}
              </label>
            ))}
          </div>
        </div>
        <div className="simops-field">
          <label>Launch mode</label>
          <div className="simops-radio-group">
            {simopsLaunchModes.map((mode) => (
              <label className="simops-radio" key={mode}>
                <input
                  checked={mode === launchMode}
                  onChange={() => onLaunchModeChange(mode)}
                  name="simops-launch-mode"
                  type="radio"
                />
                {mode}
              </label>
            ))}
          </div>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-runtime-limit">Runtime limit (sec)</label>
          <input
            id="simops-runtime-limit"
            max={3600}
            min={1}
            onChange={(event) => onRuntimeLimitChange(Number(event.target.value) || 120)}
            type="number"
            value={runtimeLimit}
          />
        </div>
        <div className="simops-field">
          <label htmlFor="simops-idempotency">Idempotency key (optional)</label>
          <input
            id="simops-idempotency"
            onChange={(event) => onIdempotencyChange(event.target.value)}
            placeholder="click once, prevent accidental repeats"
            value={idempotencyKey}
          />
        </div>
        <div className="simops-actions">
          <button disabled={submitting || isRunActive} className="primary-command" onClick={onCreate} type="button">
            <Play size={16} />
            {isRunActive ? "Run in progress" : activeRun ? "Launch replacement" : "Launch run"}
          </button>
          <button
            className="secondary-command"
            disabled={!activeRun || isRunComplete || submitting}
            onClick={onStop}
            type="button"
          >
            <Square size={16} />
            Stop run
          </button>
        </div>
        <p className="simops-message">{message}</p>
        {error && <p className="simops-error">{error}</p>}
      </div>

      <div className="panel simops-details">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Run state</p>
            <h2>Lifecycle and worker status</h2>
          </div>
        </div>
        {!activeRun ? (
          <p className="simops-placeholder">No active SimOps run yet.</p>
        ) : (
          <>
            <div className="simops-run-header">
              <span>
                <strong>run:</strong> {activeRun.run_id}
              </span>
              <StatusPill label={activeRun.lifecycle} state={activeRun.lifecycle} />
            </div>
            <div className="simops-meta">
              <span>scenario: {activeRun.scenario_id}</span>
              <span>source: {activeRun.source}</span>
              <span>mode: {activeRun.launch_mode}</span>
              <span>created: {new Date(activeRun.created_at).toLocaleString()}</span>
              <span>updated: {new Date(activeRun.updated_at).toLocaleString()}</span>
            </div>
            <div className="simops-subsection">
              <h3>Workers</h3>
              <div className="simops-worker-grid">
                {activeRun.workers.map((worker) => (
                  <article className="simops-worker-card" key={`${activeRun.run_id}-${worker.worker_id}`}>
                    <div className="simops-worker-title">
                      <span className="record-id">{worker.worker_id}</span>
                      <StatusPill label={worker.lifecycle} state={worker.lifecycle} />
                    </div>
                    <p>
                      kind: {worker.worker_kind} • frames: {worker.frames} • mode: {worker.launch_mode}
                    </p>
                    <p>endpoint: {worker.endpoint || "n/a"}</p>
                    <p>updated: {new Date(worker.updated_at).toLocaleString()}</p>
                  </article>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Latest spool commands</h3>
              <div className="simops-log">
                {activeRun.spool_commands.map((command) => (
                  <p key={command.command_id}>
                    {command.state} {command.worker_id}: {command.message}
                  </p>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Artifacts</h3>
              <div className="simops-log">
                {activeRun.artifacts.map((artifact) => (
                  <p key={artifact.artifact_id}>
                    {artifact.status} {artifact.kind} @ {artifact.location}
                  </p>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Events</h3>
              <div className="simops-log">
                {events.length === 0 ? (
                  <p>No persisted events yet.</p>
                ) : (
                  events.slice(-8).map((event, index) => (
                    <p key={`${event.run_id}-${event.event_type}-${event.occurred_at}-${index}`}>
                      {event.event_type}
                      {event.worker_id ? ` ${event.worker_id}` : ""}
                      {event.lifecycle ? ` ${event.lifecycle}` : ""} @{" "}
                      {new Date(event.occurred_at).toLocaleTimeString()}
                    </p>
                  ))
                )}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>MoQ subscription</h3>
              <div className="simops-log">
                <p>protocol: {activeRun.moq_subscription.protocol}</p>
                <p>endpoint: {activeRun.moq_subscription.endpoint}</p>
                <p>namespace: {activeRun.moq_subscription.namespace}</p>
                <p>token: {activeRun.moq_subscription.token}</p>
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
