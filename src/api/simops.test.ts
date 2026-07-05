import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createSimopsRun,
  getSimopsRun,
  listSimopsEvents,
  stopSimopsRun,
  type SimopsRunResponse
} from "./simops";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

describe("simops api client", () => {
  it("creates a run with the expected payload", async () => {
    const run = sampleRun();
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(run));
    globalThis.fetch = fetchMock;

    const response = await createSimopsRun({
      scenario_id: "scheduler-drift",
      source: "frontend",
      work_script: "scheduler-drift",
      launch_mode: "auto",
      worker_kinds: ["scheduler"],
      runtime_limit_sec: 120
    });

    expect(response.run_id).toBe("RUN-API-001");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/simops/runs",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"scenario_id":"scheduler-drift"')
      })
    );
  });

  it("reads run state, stop responses, and events", async () => {
    const run = sampleRun();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(run))
      .mockResolvedValueOnce(jsonResponse({ ...run, lifecycle: "stopped" }))
      .mockResolvedValueOnce(jsonResponse({
        run_id: run.run_id,
        events: [{ run_id: run.run_id, event_type: "run.lifecycle", lifecycle: "streaming", occurred_at: run.updated_at }]
      }));
    globalThis.fetch = fetchMock;

    await expect(getSimopsRun(run.run_id)).resolves.toMatchObject({ run_id: run.run_id });
    await expect(stopSimopsRun(run.run_id)).resolves.toMatchObject({ lifecycle: "stopped" });
    await expect(listSimopsEvents(run.run_id)).resolves.toMatchObject({ events: [{ event_type: "run.lifecycle" }] });

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/simops/runs/RUN-API-001", expect.objectContaining({ method: "GET" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/simops/runs/RUN-API-001/stop", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenNthCalledWith(3, "/api/simops/runs/RUN-API-001/events", expect.objectContaining({ method: "GET" }));
  });

  it("includes response bodies in thrown errors", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response("backend exploded", { status: 502 }));

    await expect(getSimopsRun("RUN-BAD")).rejects.toThrow("backend exploded");
  });
});

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: {
      "Content-Type": "application/json"
    }
  });
}

function sampleRun(): SimopsRunResponse {
  return {
    run_id: "RUN-API-001",
    scenario_id: "scheduler-drift",
    lifecycle: "streaming",
    source: "frontend",
    launch_mode: "auto",
    runtime_limit_sec: 120,
    created: true,
    submitted_by: "local-dev",
    created_at: "2026-07-05T12:00:00.000Z",
    updated_at: "2026-07-05T12:00:01.000Z",
    moq_subscription: {
      protocol: "moq-webtransport",
      endpoint: "https://127.0.0.1:9443/moq/simops",
      namespace: "radiant/simops/RUN-API-001",
      token: "stream-token",
      expires_at: "2026-07-05T12:15:00.000Z",
      tracks: []
    },
    workers: [],
    spool_commands: [],
    artifacts: []
  };
}
