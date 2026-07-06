import { existsSync, readFileSync } from "node:fs";

const schemaRoot = "docs/schemas/simulation-ops";
const exampleRoot = "examples/simulation-ops";

const manifestPath = `${exampleRoot}/run-manifest.scheduler-drift.json`;
const telemetryPath = `${exampleRoot}/telemetry.scheduler-drift.ndjson`;
const resultsPath = `${exampleRoot}/results.scheduler-drift.ndjson`;
const summaryPath = `${exampleRoot}/run-summary.scheduler-drift.json`;

const envelopeSchemaPath = `${schemaRoot}/simops-telemetry-envelope.v1.schema.json`;
const resultSchemaPath = `${schemaRoot}/simops-result-envelope.v1.schema.json`;
const manifestSchemaPath = `${schemaRoot}/simops-run-manifest.v1.schema.json`;
const summarySchemaPath = `${schemaRoot}/simops-run-summary.v1.schema.json`;

const payloadSchemaPaths = {
  schedulerCoScheduling: `${schemaRoot}/payload.scheduler-co-scheduling.v1.schema.json`,
  checkpointStorage: `${schemaRoot}/payload.checkpoint-storage.v1.schema.json`,
  elasticBursting: `${schemaRoot}/payload.elastic-bursting.v1.schema.json`,
  fabricProfiler: `${schemaRoot}/payload.fabric-profiler.v1.schema.json`
};

const workerKindByPayloadType = {
  schedulerCoScheduling: "scheduler",
  checkpointStorage: "storage",
  elasticBursting: "burst",
  fabricProfiler: "fabric"
};

const problems = [];

const manifestSchema = readJson(manifestSchemaPath);
const envelopeSchema = readJson(envelopeSchemaPath);
const resultSchema = readJson(resultSchemaPath);
const summarySchema = readJson(summarySchemaPath);
const payloadSchemas = Object.fromEntries(
  Object.entries(payloadSchemaPaths).map(([payloadType, path]) => [payloadType, readJson(path)])
);

const manifest = readJson(manifestPath);
const telemetry = readNdjson(telemetryPath);
const results = readNdjson(resultsPath);
const summary = readJson(summaryPath);

validateAgainstSchema(manifest, manifestSchema, "manifest");
validateAgainstSchema(summary, summarySchema, "summary");

for (const [index, frame] of telemetry.entries()) {
  const label = `telemetry line ${index + 1}`;
  validateAgainstSchema(frame, envelopeSchema, label);

  const payloadSchema = payloadSchemas[frame.payloadType];
  if (!payloadSchema) {
    problems.push(`${label}: unknown payloadType ${String(frame.payloadType)}`);
    continue;
  }

  validateAgainstSchema(frame.payload, payloadSchema, `${label}.payload`);
}
for (const [index, result] of results.entries()) {
  const label = `result line ${index + 1}`;
  validateAgainstSchema(result, resultSchema, label);
}

validateManifestSemantics(manifest);
validateTelemetrySemantics(manifest, telemetry);
validateResultSemantics(manifest, telemetry, results);
validateSummarySemantics(manifest, telemetry, summary);

if (problems.length) {
  console.error("Simulation Ops contract check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log(
  `Simulation Ops contract check passed: ${telemetry.length} telemetry frames, ${results.length} simulated result frames, ${Object.keys(payloadSchemas).length} payload schemas.`
);

function readJson(path) {
  if (!existsSync(path)) {
    problems.push(`Missing file: ${path}`);
    return {};
  }

  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    problems.push(`${path}: invalid JSON (${error.message})`);
    return {};
  }
}

function readNdjson(path) {
  if (!existsSync(path)) {
    problems.push(`Missing file: ${path}`);
    return [];
  }

  const lines = readFileSync(path, "utf8")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  return lines.flatMap((line, index) => {
    try {
      return [JSON.parse(line)];
    } catch (error) {
      problems.push(`${path}:${index + 1}: invalid JSON (${error.message})`);
      return [];
    }
  });
}

function validateAgainstSchema(value, schema, label) {
  if (!schema || typeof schema !== "object") {
    problems.push(`${label}: missing schema`);
    return;
  }

  validateNode(value, schema, label);
}

function validateNode(value, schema, path) {
  if (!schema || typeof schema !== "object") {
    return;
  }

  if (schema.type && !matchesType(value, schema.type)) {
    problems.push(`${path}: expected ${schema.type}, got ${describeType(value)}`);
    return;
  }

  if (schema.enum && !schema.enum.includes(value)) {
    problems.push(`${path}: expected one of ${schema.enum.join(", ")}, got ${JSON.stringify(value)}`);
  }

  if (schema.pattern && typeof value === "string") {
    const pattern = new RegExp(schema.pattern);
    if (!pattern.test(value)) {
      problems.push(`${path}: does not match pattern ${schema.pattern}`);
    }
  }

  if (schema.format === "date-time" && typeof value === "string" && Number.isNaN(Date.parse(value))) {
    problems.push(`${path}: invalid date-time ${JSON.stringify(value)}`);
  }

  if (typeof value === "number") {
    if (typeof schema.minimum === "number" && value < schema.minimum) {
      problems.push(`${path}: ${value} is below minimum ${schema.minimum}`);
    }
    if (typeof schema.maximum === "number" && value > schema.maximum) {
      problems.push(`${path}: ${value} is above maximum ${schema.maximum}`);
    }
  }

  if (Array.isArray(value)) {
    if (typeof schema.minItems === "number" && value.length < schema.minItems) {
      problems.push(`${path}: has ${value.length} items, expected at least ${schema.minItems}`);
    }
    if (typeof schema.maxItems === "number" && value.length > schema.maxItems) {
      problems.push(`${path}: has ${value.length} items, expected at most ${schema.maxItems}`);
    }
    if (schema.items) {
      value.forEach((item, index) => validateNode(item, schema.items, `${path}[${index}]`));
    }
  }

  if (isPlainObject(value)) {
    const properties = schema.properties ?? {};
    const required = schema.required ?? [];

    for (const field of required) {
      if (!(field in value)) {
        problems.push(`${path}: missing required field ${field}`);
      }
    }

    if (schema.additionalProperties === false) {
      for (const field of Object.keys(value)) {
        if (!(field in properties)) {
          problems.push(`${path}: unexpected field ${field}`);
        }
      }
    }

    for (const [field, fieldSchema] of Object.entries(properties)) {
      if (field in value) {
        validateNode(value[field], fieldSchema, `${path}.${field}`);
      }
    }
  }
}

function matchesType(value, type) {
  switch (type) {
    case "object":
      return isPlainObject(value);
    case "array":
      return Array.isArray(value);
    case "string":
      return typeof value === "string";
    case "integer":
      return Number.isInteger(value);
    case "number":
      return typeof value === "number" && Number.isFinite(value);
    case "boolean":
      return typeof value === "boolean";
    default:
      problems.push(`schema: unsupported type ${type}`);
      return true;
  }
}

function describeType(value) {
  if (Array.isArray(value)) {
    return "array";
  }
  if (value === null) {
    return "null";
  }
  return typeof value;
}

function isPlainObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validateManifestSemantics(candidate) {
  const workers = candidate.workers ?? [];
  const workerIds = new Set();

  for (const worker of workers) {
    if (workerIds.has(worker.workerId)) {
      problems.push(`manifest.workers: duplicate workerId ${worker.workerId}`);
    }
    workerIds.add(worker.workerId);

    const expectedKind = workerKindByPayloadType[worker.payloadType];
    if (expectedKind && worker.workerKind !== expectedKind) {
      problems.push(
        `manifest.workers.${worker.workerId}: payloadType ${worker.payloadType} expects workerKind ${expectedKind}`
      );
    }
  }

  const curve = candidate.randomization?.pressureCurve ?? [];
  let previousOffset = -1;
  for (const point of curve) {
    if (point.offsetSec <= previousOffset) {
      problems.push("manifest.randomization.pressureCurve: offsetSec values must increase");
    }
    previousOffset = point.offsetSec;
  }

  for (const baseline of candidate.randomization?.baseline ?? []) {
    if (baseline.nominalMin > baseline.nominalMax) {
      problems.push(`manifest.randomization.baseline.${baseline.metricPath}: nominalMin exceeds nominalMax`);
    }
  }

  for (const bound of candidate.randomization?.bounds ?? []) {
    if (bound.min > bound.max) {
      problems.push(`manifest.randomization.bounds.${bound.metricPath}: min exceeds max`);
    }
  }

  for (const artifact of candidate.artifacts ?? []) {
    if (!existsSync(artifact.path)) {
      problems.push(`manifest.artifacts.${artifact.artifactId}: missing path ${artifact.path}`);
    }
  }
}

function validateTelemetrySemantics(candidate, frames) {
  const workersById = new Map((candidate.workers ?? []).map((worker) => [worker.workerId, worker]));
  const lastSequenceByWorker = new Map();
  const manifestBounds = candidate.randomization?.bounds ?? [];

  for (const [index, frame] of frames.entries()) {
    const label = `telemetry line ${index + 1}`;
    if (frame.runId !== candidate.runId) {
      problems.push(`${label}: runId ${frame.runId} does not match manifest ${candidate.runId}`);
    }
    if (frame.scenarioId !== candidate.scenarioId) {
      problems.push(`${label}: scenarioId ${frame.scenarioId} does not match manifest ${candidate.scenarioId}`);
    }

    const worker = workersById.get(frame.workerId);
    if (!worker) {
      problems.push(`${label}: unknown workerId ${frame.workerId}`);
    } else {
      if (frame.workerKind !== worker.workerKind) {
        problems.push(`${label}: workerKind ${frame.workerKind} does not match manifest ${worker.workerKind}`);
      }
      if (frame.payloadType !== worker.payloadType) {
        problems.push(`${label}: payloadType ${frame.payloadType} does not match manifest ${worker.payloadType}`);
      }
    }

    const lastSequence = lastSequenceByWorker.get(frame.workerId) ?? 0;
    if (frame.sequence <= lastSequence) {
      problems.push(`${label}: sequence ${frame.sequence} is not greater than previous ${lastSequence}`);
    }
    lastSequenceByWorker.set(frame.workerId, frame.sequence);

    if (frame.receivedAt && Date.parse(frame.receivedAt) < Date.parse(frame.emittedAt)) {
      problems.push(`${label}: receivedAt is before emittedAt`);
    }

    for (const bound of manifestBounds) {
      const value = getPath(frame, bound.metricPath);
      if (value == null) {
        continue;
      }
      if (typeof value !== "number") {
        problems.push(`${label}.${bound.metricPath}: expected numeric metric`);
        continue;
      }
      if (value < bound.min || value > bound.max) {
        problems.push(`${label}.${bound.metricPath}: ${value} is outside manifest bounds ${bound.min}..${bound.max}`);
      }
    }
  }
}

function validateResultSemantics(candidate, telemetryFrames, results) {
  const telemetryWorkers = new Map((candidate.workers ?? []).map((worker) => [worker.workerId, worker]));
  const telemetryWorkerSequences = new Map();
  for (const frame of telemetryFrames) {
    const sequences = telemetryWorkerSequences.get(frame.workerId) ?? new Set();
    sequences.add(frame.sequence);
    telemetryWorkerSequences.set(frame.workerId, sequences);
  }

  const lastSequenceByWorker = new Map();
  for (const [index, result] of results.entries()) {
    const label = `result line ${index + 1}`;
    if (result.schemaVersion !== "simops.result.v1") {
      problems.push(`${label}: simulated result must use simops.result.v1, not telemetry`);
    }
    if (result.valueBasis !== "simulated") {
      problems.push(`${label}: worker-produced result must stay valueBasis=simulated`);
    }
    if (result.valueBasis === "imputed") {
      problems.push(`${label}: imputed state must be produced by the digital twin projector, not a SimOps worker`);
    }
    if (result.runId !== candidate.runId) {
      problems.push(`${label}: runId ${result.runId} does not match manifest ${candidate.runId}`);
    }
    if (result.scenarioId !== candidate.scenarioId) {
      problems.push(`${label}: scenarioId ${result.scenarioId} does not match manifest ${candidate.scenarioId}`);
    }

    const worker = telemetryWorkers.get(result.workerId);
    if (!worker) {
      problems.push(`${label}: unknown workerId ${result.workerId}`);
    } else if (result.workerKind !== worker.workerKind) {
      problems.push(`${label}: workerKind ${result.workerKind} does not match manifest ${worker.workerKind}`);
    }

    const lastSequence = lastSequenceByWorker.get(result.workerId) ?? 0;
    if (result.sequence <= lastSequence) {
      problems.push(`${label}: sequence ${result.sequence} is not greater than previous ${lastSequence}`);
    }
    lastSequenceByWorker.set(result.workerId, result.sequence);

    const telemetrySequences = telemetryWorkerSequences.get(result.workerId);
    if (telemetrySequences && !telemetrySequences.has(result.sequence)) {
      problems.push(`${label}: no matching telemetry unit for worker ${result.workerId} sequence ${result.sequence}`);
    }

    if (Date.parse(result.inputWindow.start) > Date.parse(result.inputWindow.end)) {
      problems.push(`${label}: inputWindow.start is after inputWindow.end`);
    }
    if (result.receivedAt && Date.parse(result.receivedAt) < Date.parse(result.producedAt)) {
      problems.push(`${label}: receivedAt is before producedAt`);
    }

    for (const value of result.values ?? []) {
      if (!value.valueId || !value.resultId) {
        problems.push(`${label}: result values must carry resultId and valueId`);
      }
    }
  }
}

function validateSummarySemantics(candidate, frames, runSummary) {
  if (runSummary.runId !== candidate.runId) {
    problems.push(`summary: runId ${runSummary.runId} does not match manifest ${candidate.runId}`);
  }
  if (runSummary.scenarioId !== candidate.scenarioId) {
    problems.push(`summary: scenarioId ${runSummary.scenarioId} does not match manifest ${candidate.scenarioId}`);
  }
  if (runSummary.frameCount !== frames.length) {
    problems.push(`summary: frameCount ${runSummary.frameCount} does not match telemetry count ${frames.length}`);
  }

  const manifestWorkers = new Map((candidate.workers ?? []).map((worker) => [worker.workerId, worker]));
  const frameCounts = new Map();
  for (const frame of frames) {
    frameCounts.set(frame.workerId, (frameCounts.get(frame.workerId) ?? 0) + 1);
  }

  for (const workerSummary of runSummary.workers ?? []) {
    const manifestWorker = manifestWorkers.get(workerSummary.workerId);
    if (!manifestWorker) {
      problems.push(`summary.workers.${workerSummary.workerId}: missing from manifest`);
      continue;
    }
    if (workerSummary.workerKind !== manifestWorker.workerKind) {
      problems.push(`summary.workers.${workerSummary.workerId}: workerKind mismatch`);
    }
    if (workerSummary.payloadType !== manifestWorker.payloadType) {
      problems.push(`summary.workers.${workerSummary.workerId}: payloadType mismatch`);
    }
    const observedFrames = frameCounts.get(workerSummary.workerId) ?? 0;
    if (workerSummary.frames !== observedFrames) {
      problems.push(
        `summary.workers.${workerSummary.workerId}: frames ${workerSummary.frames} does not match telemetry ${observedFrames}`
      );
    }
  }

  const boundsByPath = new Map((candidate.randomization?.bounds ?? []).map((bound) => [bound.metricPath, bound]));
  for (const metric of runSummary.aggregateMetrics ?? []) {
    if (!(metric.min <= metric.avg && metric.avg <= metric.max)) {
      problems.push(`summary.aggregateMetrics.${metric.metricPath}: expected min <= avg <= max`);
    }
    const bound = boundsByPath.get(metric.metricPath);
    if (bound && (metric.min < bound.min || metric.max > bound.max)) {
      problems.push(`summary.aggregateMetrics.${metric.metricPath}: outside manifest bounds ${bound.min}..${bound.max}`);
    }
  }

  for (const interval of runSummary.degradedIntervals ?? []) {
    if (interval.startOffsetSec > interval.endOffsetSec) {
      problems.push(`summary.degradedIntervals.${interval.reason}: startOffsetSec exceeds endOffsetSec`);
    }
  }

  for (const artifact of runSummary.artifacts ?? []) {
    if (!existsSync(artifact.path)) {
      problems.push(`summary.artifacts.${artifact.artifactId}: missing path ${artifact.path}`);
    }
  }
}

function getPath(value, dottedPath) {
  return dottedPath.split(".").reduce((current, segment) => {
    if (current == null) {
      return undefined;
    }
    return current[segment];
  }, value);
}
