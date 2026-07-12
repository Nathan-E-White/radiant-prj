#!/usr/bin/env node
import { readFileSync } from "node:fs";

const command = process.argv[2];
const args = process.argv.slice(3);
const raw = readFileSync(0, "utf8");

let parsed;
try {
  parsed = JSON.parse(raw);
} catch {
  process.exit(1);
}

switch (command) {
  case "run-id": {
    const runID = typeof parsed.run_id === "string" ? parsed.run_id.trim() : "";
    if (!runID) {
      process.exit(1);
    }
    process.stdout.write(runID);
    break;
  }
  case "status-ready": {
    const workers = Array.isArray(parsed.workers) ? parsed.workers : [];
    const artifacts = Array.isArray(parsed.artifacts) ? parsed.artifacts : [];
    const frames = workers.reduce((sum, worker) => sum + numeric(worker.frames), 0);
    const committed = artifacts.some((artifact) => artifact?.status === "committed");
    process.exit(frames > 0 && committed ? 0 : 1);
    break;
  }
  case "events-nonempty": {
    process.exit(Array.isArray(parsed.events) && parsed.events.length > 0 ? 0 : 1);
    break;
  }
  case "runtime-worker": {
    runtimeWorker(parsed, args);
    break;
  }
  case "container-proof": {
    containerProof(parsed);
    break;
  }
  default:
    console.error("Usage: simops-smoke-json.mjs run-id|status-ready|events-nonempty|runtime-worker|container-proof");
    process.exit(2);
}

function numeric(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function runtimeWorker(payload, options) {
  const expectedState = typeof options[0] === "string" ? options[0].trim() : "";
  const requireFrames = options.includes("--frames");
  const runtimeIndex = options.indexOf("--runtime");
  const expectedRuntime = runtimeIndex >= 0 ? options[runtimeIndex + 1] : "docker";
  if (!expectedState) {
    console.error("runtime-worker requires an observed lifecycle state.");
    process.exit(2);
  }
  if (!new Set(["docker", "kubernetes"]).has(expectedRuntime)) {
    console.error("runtime-worker --runtime must be docker or kubernetes.");
    process.exit(2);
  }
  const workers = Array.isArray(payload.workers) ? payload.workers : [];
  const worker = workers.find((item) => item?.observed_lifecycle === expectedState);
  if (!worker) {
    const observed = workers.map((item) => item?.observed_lifecycle || "<missing>").join(", ");
    console.error(`No worker has observed_lifecycle=${expectedState}; observed_lifecycle values: ${observed}`);
    process.exit(1);
  }
  if (worker.runtime !== expectedRuntime) {
    console.error(`Expected ${expectedRuntime} runtime worker, got runtime=${worker.runtime || "<missing>"}.`);
    process.exit(1);
  }
  if (typeof worker.runtime_id !== "string" || worker.runtime_id.trim() === "") {
    console.error(`Expected runtime_id for observed ${expectedRuntime} worker.`);
    process.exit(1);
  }
  const frames = numeric(worker.frames);
  if (requireFrames && frames < 1) {
    console.error(`Expected at least one ingested frame for ${worker.worker_id || "<unknown-worker>"}.`);
    process.exit(1);
  }
  process.stdout.write(
    [
      "Runtime worker proof:",
      `run=${payload.run_id || "<unknown-run>"}`,
      `worker=${worker.worker_id || "<unknown-worker>"}`,
      `state=${worker.observed_lifecycle}`,
      `runtime=${worker.runtime}`,
      `runtime_id=${worker.runtime_id}`,
      `frames=${frames}`,
    ].join(" "),
  );
}

function containerProof(payload) {
  const container = Array.isArray(payload) ? payload[0] : payload;
  if (!container || typeof container !== "object") {
    console.error("container-proof expects docker inspect JSON.");
    process.exit(1);
  }
  const config = container.Config || {};
  const labels = config.Labels || {};
  const env = parseEnv(config.Env);
  if (labels["simops.role"] !== "ordinary-worker") {
    console.error(`Expected ordinary worker container, got simops.role=${labels["simops.role"] || "<missing>"}.`);
    process.exit(1);
  }
  if (labels["simops.runtime_adapter"] !== "docker-sdk") {
    console.error(`Expected docker-sdk runtime adapter, got ${labels["simops.runtime_adapter"] || "<missing>"}.`);
    process.exit(1);
  }
  for (const key of [
    "SIMOPS_INGEST_URL",
    "SIMOPS_INGEST_TOKEN",
    "SIMOPS_RESULT_INGEST_URL",
    "SIMOPS_RESULT_INGEST_TOKEN",
  ]) {
    if (!env.has(key) || env.get(key).trim() === "") {
      console.error(`Missing required worker gateway ingest env ${key}.`);
      process.exit(1);
    }
  }
  if (!env.get("SIMOPS_INGEST_URL").includes("/internal/simops/runs/") || !env.get("SIMOPS_INGEST_URL").includes("/ingest")) {
    console.error("SIMOPS_INGEST_URL does not target gateway ingest.");
    process.exit(1);
  }
  if (!env.get("SIMOPS_RESULT_INGEST_URL").includes("/internal/simops/runs/") || !env.get("SIMOPS_RESULT_INGEST_URL").includes("/results")) {
    console.error("SIMOPS_RESULT_INGEST_URL does not target gateway result ingest.");
    process.exit(1);
  }
  const forbidden = [...env.keys()].filter(isDirectDataPlaneKey);
  if (forbidden.length > 0) {
    console.error(`Ordinary worker exposes direct data-plane credentials/env: ${forbidden.join(", ")}`);
    process.exit(1);
  }

  const proof = [
    `container=${container.Id || "<unknown-container>"}`,
    `worker=${labels["simops.worker_id"] || env.get("SIMOPS_WORKER_ID") || "<unknown-worker>"}`,
    `image=${config.Image || "<unknown-image>"}`,
    `SIMOPS_INGEST_URL=${env.get("SIMOPS_INGEST_URL")}`,
    "SIMOPS_INGEST_TOKEN=<redacted>",
    `SIMOPS_RESULT_INGEST_URL=${env.get("SIMOPS_RESULT_INGEST_URL")}`,
    "SIMOPS_RESULT_INGEST_TOKEN=<redacted>",
  ].join(" ");
  process.stdout.write(`Docker worker gateway-ingest-only proof: ${proof}`);
}

function parseEnv(values) {
  const env = new Map();
  for (const rawValue of Array.isArray(values) ? values : []) {
    const index = rawValue.indexOf("=");
    if (index <= 0) {
      continue;
    }
    env.set(rawValue.slice(0, index), rawValue.slice(index + 1));
  }
  return env;
}

function isDirectDataPlaneKey(key) {
  return (
    key.startsWith("SIMOPS_REDPANDA_") ||
    key.startsWith("SIMOPS_POSTGRES_") ||
    key.startsWith("SIMOPS_ICEBERG_") ||
    key.startsWith("WORKBENCH_") ||
    key.startsWith("AWS_") ||
    key === "DOCKER_HOST" ||
    key === "KUBECONFIG" ||
    key.startsWith("KUBERNETES_")
  );
}
