import { existsSync, readFileSync } from "node:fs";

const schemaRoot = "docs/schemas";
const exampleRoot = "examples";

const paths = {
  valueBasisSchema: `${schemaRoot}/simulator-workbench/value-basis.v1.schema.json`,
  sourceSchema: `${schemaRoot}/scada/resident-source-declaration.v1.schema.json`,
  telemetrySchema: `${schemaRoot}/scada/scada-telemetry.v1.schema.json`,
  twinSchema: `${schemaRoot}/digital-twin/digital-twin-state.v1.schema.json`,
  lineageSchema: `${schemaRoot}/digital-twin/value-lineage.v1.schema.json`,
  workbenchSchema: `${schemaRoot}/simulator-workbench/workbench-state.v1.schema.json`,
  sourceExample: `${exampleRoot}/scada/resident-sources.mixed.json`,
  telemetryExample: `${exampleRoot}/scada/telemetry.mixed.ndjson`,
  twinExample: `${exampleRoot}/digital-twin/twin-state.mixed.json`,
  lineageExample: `${exampleRoot}/digital-twin/value-lineage.core-margin.json`,
  workbenchExample: `${exampleRoot}/simulator-workbench/workbench-state.mixed.json`
};

const expectedSignalKinds = new Set([
  "flux",
  "temperature",
  "pressure",
  "actuatorState",
  "electricalState",
  "comms"
]);
const valueBasisValues = ["measured", "imputed", "simulated"];
const problems = [];

const valueBasisSchema = readJson(paths.valueBasisSchema);
const sourceSchema = readJson(paths.sourceSchema);
const telemetrySchema = readJson(paths.telemetrySchema);
const twinSchema = readJson(paths.twinSchema);
const lineageSchema = readJson(paths.lineageSchema);
const workbenchSchema = readJson(paths.workbenchSchema);

const source = readJson(paths.sourceExample);
const telemetry = readNdjson(paths.telemetryExample);
const twin = readJson(paths.twinExample);
const lineage = readJson(paths.lineageExample);
const workbench = readJson(paths.workbenchExample);

for (const basis of valueBasisValues) {
  validateAgainstSchema(basis, valueBasisSchema, `valueBasis.${basis}`);
}
validateAgainstSchema(source, sourceSchema, "resident source example");
for (const [index, frame] of telemetry.entries()) {
  validateAgainstSchema(frame, telemetrySchema, `telemetry line ${index + 1}`);
}
validateAgainstSchema(twin, twinSchema, "twin state example");
validateAgainstSchema(lineage, lineageSchema, "lineage example");
validateAgainstSchema(workbench, workbenchSchema, "workbench state example");

validateSourceCoverage(source);
validateTelemetrySemantics(source, telemetry);
validateTwinSemantics(twin, lineage, workbench);
validateWorkbenchRefs(workbench);

if (problems.length) {
  console.error("Simulator Workbench contract check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log(
  `Simulator Workbench contract check passed: ${telemetry.length} measured frames, ${countTwinValues(twin)} twin values, ${workbench.panels?.length ?? 0} panels.`
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

  return readFileSync(path, "utf8")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .flatMap((line, index) => {
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

function validateSourceCoverage(source) {
  const declaredKinds = new Set((source.tags ?? []).map((tag) => tag.signalKind));
  for (const kind of expectedSignalKinds) {
    if (!declaredKinds.has(kind)) {
      problems.push(`resident source missing expected mixed signal kind ${kind}`);
    }
  }

  for (const tag of source.tags ?? []) {
    if (tag.valueBasis !== "measured") {
      problems.push(`resident source tag ${tag.tagId} must stay valueBasis=measured`);
    }
  }
}

function validateTelemetrySemantics(source, telemetry) {
  const declaredTags = new Map((source.tags ?? []).map((tag) => [tag.tagId, tag]));
  const previousSequenceByTag = new Map();

  for (const frame of telemetry) {
    const declaration = declaredTags.get(frame.tagId);
    if (!declaration) {
      problems.push(`telemetry frame references undeclared tag ${frame.tagId}`);
      continue;
    }
    if (declaration.signalKind !== frame.signalKind) {
      problems.push(`telemetry frame ${frame.tagId} signalKind ${frame.signalKind} does not match declaration ${declaration.signalKind}`);
    }
    if (frame.valueBasis !== "measured") {
      problems.push(`telemetry frame ${frame.tagId} must stay valueBasis=measured`);
    }
    if (frame.syntheticStatus !== "public-safe-standin") {
      problems.push(`telemetry frame ${frame.tagId} must keep public-safe synthetic status`);
    }

    const key = `${frame.sourceId}:${frame.tagId}`;
    const previous = previousSequenceByTag.get(key) ?? -1;
    if (frame.sequence <= previous) {
      problems.push(`telemetry frame ${frame.tagId} sequence must increase per source/tag`);
    }
    previousSequenceByTag.set(key, frame.sequence);
  }
}

function validateTwinSemantics(twin, lineage, workbench) {
  const values = flattenTwinValues(twin);
  const valueBasisCounts = countByBasis(values);
  for (const basis of valueBasisValues) {
    if ((valueBasisCounts[basis] ?? 0) === 0) {
      problems.push(`twin state must include at least one ${basis} value`);
    }
  }

  const lineagedValue = values.find((value) => value.valueId === lineage.valueId);
  if (!lineagedValue) {
    problems.push(`lineage ${lineage.lineageId} references missing twin value ${lineage.valueId}`);
  } else if (lineagedValue.valueBasis !== lineage.valueBasis) {
    problems.push(`lineage ${lineage.lineageId} basis ${lineage.valueBasis} does not match twin value ${lineagedValue.valueBasis}`);
  }

  const lineageInputBasis = new Set((lineage.inputs ?? []).map((input) => input.valueBasis));
  for (const basis of valueBasisValues) {
    if (!lineageInputBasis.has(basis)) {
      problems.push(`lineage ${lineage.lineageId} should show at least one ${basis} input for the first mixed demo`);
    }
  }

  const expectedSummary = workbench.valueBasisSummary ?? {};
  for (const basis of valueBasisValues) {
    if (expectedSummary[basis] !== (valueBasisCounts[basis] ?? 0)) {
      problems.push(`workbench valueBasisSummary.${basis}=${expectedSummary[basis]} does not match twin count ${valueBasisCounts[basis] ?? 0}`);
    }
  }
}

function validateWorkbenchRefs(workbench) {
  for (const ref of [...(workbench.measuredStateRefs ?? []), workbench.twinStateRef, ...(workbench.lineageRefs ?? [])]) {
    if (typeof ref === "string" && !existsSync(ref)) {
      problems.push(`workbench reference does not exist: ${ref}`);
    }
  }

  const panelBasis = new Set((workbench.panels ?? []).map((panel) => panel.valueBasis));
  for (const basis of valueBasisValues) {
    if (!panelBasis.has(basis)) {
      problems.push(`workbench panels must include ${basis} value basis`);
    }
  }
}

function flattenTwinValues(twin) {
  return (twin.entities ?? []).flatMap((entity) => entity.values ?? []);
}

function countTwinValues(twin) {
  return flattenTwinValues(twin).length;
}

function countByBasis(values) {
  return values.reduce((counts, value) => {
    counts[value.valueBasis] = (counts[value.valueBasis] ?? 0) + 1;
    return counts;
  }, {});
}
