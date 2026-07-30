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
  case "snapshot-ready":
    process.exit(snapshotReady(parsed) ? 0 : 1);
    break;
  default:
    console.error("Usage: workbench-dataflow-json.mjs snapshot-ready");
    process.exit(2);
}

function snapshotReady(snapshot) {
  const generation = snapshot?.generation;
  const state = snapshot?.state;
  const measured = Array.isArray(snapshot?.measured) ? snapshot.measured : [];
  const results = Array.isArray(snapshot?.results) ? snapshot.results : [];
  const entities = Array.isArray(snapshot?.twin?.entities) ? snapshot.twin.entities : [];
  const values = entities.flatMap((entity) => (Array.isArray(entity?.values) ? entity.values : []));
  const lineage = Array.isArray(snapshot?.lineage) ? snapshot.lineage : [];
  const imputedLineage = lineage.find((record) => record?.valueId === "VAL-IMPUTED-CORE-MARGIN");
  const inputs = Array.isArray(imputedLineage?.inputs) ? imputedLineage.inputs : [];

  return Number.isSafeInteger(generation) && generation > 0 &&
    state?.snapshotGeneration === generation &&
    measured.some((frame) => frame?.valueBasis === "measured") &&
    results.some((frame) => frame?.valueBasis === "simulated") &&
    values.some((value) => value?.valueBasis === "measured") &&
    values.some((value) => value?.valueBasis === "simulated") &&
    values.some((value) => value?.valueBasis === "imputed") &&
    imputedLineage?.valueBasis === "imputed" &&
    inputs.some((input) => input?.valueBasis === "measured") &&
    inputs.some((input) => input?.valueBasis === "simulated");
}
