import { describe, expect, it } from "vitest";
import {
  buildSimopsRunRequest,
  isSimopsRunActive,
  isSimopsRunTerminal,
  toggleSimopsWorker,
  validateSimopsRun
} from "./simopsFeature";

describe("simops feature model helpers", () => {
  it("builds request payloads with optional idempotency key", () => {
    const payload = buildSimopsRunRequest({
      scenario: "nominal",
      source: "frontend",
      launchMode: "auto",
      workerKinds: ["scheduler", "burst"],
      runtimeLimit: 500,
      idempotencyKey: "  run-key  "
    });

    expect(payload).toMatchObject({
      scenario_id: "nominal",
      source: "frontend",
      work_script: "nominal",
      launch_mode: "auto",
      worker_kinds: ["scheduler", "burst"],
      runtime_limit_sec: 500,
      idempotency_key: "run-key"
    });
  });

  it("validates worker selection before create", () => {
    expect(validateSimopsRun({ workerKinds: [], runtimeLimit: 120 })).toEqual({
      valid: false,
      message: "Choose at least one worker kind."
    });

    expect(validateSimopsRun({ workerKinds: ["scheduler"], runtimeLimit: 0 })).toEqual({
      valid: false,
      message: "Runtime limit must stay between 1 and 3600 seconds."
    });
  });

  it("toggles worker kinds without mutation", () => {
    expect(toggleSimopsWorker(["scheduler", "storage"], "scheduler")).toEqual(["storage"]);
    expect(toggleSimopsWorker(["storage"], "fabric")).toEqual(["storage", "fabric"]);
  });

  it("maps lifecycle values to active and terminal states", () => {
    expect(isSimopsRunActive("streaming")).toBe(true);
    expect(isSimopsRunActive("complete")).toBe(false);
    expect(isSimopsRunTerminal("failed")).toBe(true);
    expect(isSimopsRunTerminal("starting")).toBe(false);
  });
});
