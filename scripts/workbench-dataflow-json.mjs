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
  case "state-ready": {
    const summary = parsed?.valueBasisSummary ?? {};
    process.exit(
      numeric(summary.measured) >= 1 &&
        numeric(summary.simulated) >= 1 &&
        numeric(summary.imputed) >= 1
        ? 0
        : 1,
    );
    break;
  }
  case "lineage-ready": {
    const inputs = Array.isArray(parsed?.inputs) ? parsed.inputs : [];
    const hasMeasured = inputs.some((input) => input?.valueBasis === "measured");
    const hasSimulated = inputs.some((input) => input?.valueBasis === "simulated");
    process.exit(parsed?.valueBasis === "imputed" && hasMeasured && hasSimulated ? 0 : 1);
    break;
  }
  case "frames-ready": {
    const frames = Array.isArray(parsed?.frames) ? parsed.frames : [];
    process.exit(frames.some((frame) => frame?.valueBasis === "measured") ? 0 : 1);
    break;
  }
  case "twin-ready": {
    const entities = Array.isArray(parsed?.entities) ? parsed.entities : [];
    const values = entities.flatMap((entity) => (Array.isArray(entity?.values) ? entity.values : []));
    process.exit(
      values.some((value) => value?.valueBasis === "measured") &&
        values.some((value) => value?.valueBasis === "simulated") &&
        values.some((value) => value?.valueBasis === "imputed")
        ? 0
        : 1,
    );
    break;
  }
  default:
    console.error("Usage: workbench-dataflow-json.mjs state-ready|lineage-ready|frames-ready|twin-ready");
    process.exit(2);
}

function numeric(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
