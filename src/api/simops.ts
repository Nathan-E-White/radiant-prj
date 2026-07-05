export type SimopsLifecycle = "created" | "starting" | "streaming" | "degraded" | "complete" | "failed" | "stopped";
export type SimopsWorkerKind = "scheduler" | "storage" | "burst" | "fabric";

export type SimopsRunRequest = {
  scenario_id: string;
  source: string;
  work_script: string;
  launch_mode: "resident" | "spawn" | "auto";
  worker_kinds: SimopsWorkerKind[];
  runtime_limit_sec: number;
  idempotency_key?: string;
};

export type SimopsRunResponse = {
  run_id: string;
  scenario_id: string;
  lifecycle: SimopsLifecycle;
  source: string;
  launch_mode: string;
  runtime_limit_sec: number;
  created: boolean;
  submitted_by: string;
  created_at: string;
  updated_at: string;
  moq_subscription: SimopsMoQSubscription;
  workers: SimopsWorkerRecord[];
  spool_commands: SimopsSpoolCommand[];
  artifacts: SimopsArtifactRecord[];
};

export type SimopsWorkerRecord = {
  worker_id: string;
  worker_kind: SimopsWorkerKind;
  lifecycle: SimopsLifecycle;
  launch_mode: string;
  endpoint?: string;
  frames: number;
  updated_at: string;
};

export type SimopsSpoolCommand = {
  command_id: string;
  run_id: string;
  worker_id: string;
  mode: string;
  state: SimopsLifecycle;
  message: string;
  created_at: string;
  updated_at: string;
};

export type SimopsArtifactRecord = {
  artifact_id: string;
  run_id: string;
  kind: string;
  media_type: string;
  location: string;
  iceberg_table?: string;
  status: string;
  created_at: string;
};

export type SimopsMoQSubscription = {
  protocol: string;
  endpoint: string;
  namespace: string;
  token: string;
  expires_at: string;
  tracks: Array<{
    name: string;
    role: string;
    worker_id?: string;
    worker_kind?: string;
  }>;
};

export type SimopsEvent = {
  run_id: string;
  worker_id?: string;
  event_type: string;
  lifecycle?: SimopsLifecycle;
  frame?: unknown;
  occurred_at: string;
};

export type SimopsEventsResponse = {
  run_id: string;
  events: SimopsEvent[];
};

const SIMOPS_API_BASE = (import.meta.env.VITE_SIMOPS_API_BASE ?? "").replace(/\/$/, "");

function simopsApiUrl(path: string): string {
  return `${SIMOPS_API_BASE}${path}`;
}

export async function createSimopsRun(payload: SimopsRunRequest): Promise<SimopsRunResponse> {
  return readJsonResponse<SimopsRunResponse>(
    await fetch(simopsApiUrl("/api/simops/runs"), {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json"
      },
      body: JSON.stringify(payload)
    })
  );
}

export async function getSimopsRun(runID: string): Promise<SimopsRunResponse> {
  return readJsonResponse<SimopsRunResponse>(
    await fetch(simopsApiUrl(`/api/simops/runs/${encodeURIComponent(runID)}`), {
      method: "GET",
      headers: {
        Accept: "application/json"
      }
    })
  );
}

export async function stopSimopsRun(runID: string): Promise<SimopsRunResponse> {
  return readJsonResponse<SimopsRunResponse>(
    await fetch(simopsApiUrl(`/api/simops/runs/${encodeURIComponent(runID)}/stop`), {
      method: "POST",
      headers: {
        Accept: "application/json"
      }
    })
  );
}

export async function listSimopsEvents(runID: string): Promise<SimopsEventsResponse> {
  return readJsonResponse<SimopsEventsResponse>(
    await fetch(simopsApiUrl(`/api/simops/runs/${encodeURIComponent(runID)}/events`), {
      method: "GET",
      headers: {
        Accept: "application/json"
      }
    })
  );
}

async function readJsonResponse<T>(response: Response): Promise<T> {
  const raw = await response.text();
  if (!response.ok) {
    throw new Error(`request failed (${response.status}): ${raw}`);
  }
  return JSON.parse(raw) as T;
}
