#!/usr/bin/env node
import { readFileSync } from "node:fs";

const command = process.argv[2];
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
  default:
    console.error("Usage: simops-smoke-json.mjs run-id|status-ready|events-nonempty");
    process.exit(2);
}

function numeric(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
