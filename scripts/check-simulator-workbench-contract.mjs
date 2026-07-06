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
  lineageExamples: [
    `${exampleRoot}/digital-twin/value-lineage.core-distribution.json`,
    `${exampleRoot}/digital-twin/value-lineage.core-margin.json`,
    `${exampleRoot}/digital-twin/value-lineage.cooldown-heat.json`
  ],
  workbenchExample: `${exampleRoot}/simulator-workbench/workbench-state.mixed.json`,
  fleetUnitsExample: `${exampleRoot}/simulator-workbench/fleet-units.mixed.json`,
  commercialBasisExample: `${exampleRoot}/simulator-workbench/commercial-display-basis.mixed.json`
};

const expectedSignalKinds = new Set([
  "flux",
  "temperature",
  "pressure",
  "actuatorState",
  "electricalState",
  "flow"
]);
const valueBasisValues = ["measured", "imputed", "simulated"];
const allowedAvailabilityPhases = new Set([
  "online generation",
  "ramping",
  "cooldown",
  "planned maintenance outage",
  "unplanned maintenance outage",
  "refueling outage"
]);
const allowedCommercialModes = new Set([
  "PPA electric",
  "direct unit sale",
  "facility heat",
  "desalination heat",
  "resilience backup"
]);
const expectedCommercialExclusions = ["not billing", "not settlement", "not tariff", "not market-cleared", "not dispatch"];
const bannedCommercialTerms = [
  "lease",
  "revenue",
  "invoice",
  "lmp",
  "bid",
  "offer",
  "capacity payment",
  "lost-generation",
  "lost generation",
  "outage cost"
];
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
const lineages = paths.lineageExamples.map((path) => readJson(path));
const workbench = readJson(paths.workbenchExample);
const fleetUnits = readJson(paths.fleetUnitsExample);
const commercialBasis = readJson(paths.commercialBasisExample);

for (const basis of valueBasisValues) {
  validateAgainstSchema(basis, valueBasisSchema, `valueBasis.${basis}`);
}
validateAgainstSchema(source, sourceSchema, "resident source example");
for (const [index, frame] of telemetry.entries()) {
  validateAgainstSchema(frame, telemetrySchema, `telemetry line ${index + 1}`);
}
validateAgainstSchema(twin, twinSchema, "twin state example");
for (const [index, lineage] of lineages.entries()) {
  validateAgainstSchema(lineage, lineageSchema, `lineage example ${index + 1}`);
}
validateAgainstSchema(workbench, workbenchSchema, "workbench state example");

validateSourceCoverage(source);
validateTelemetrySemantics(source, telemetry);
validateTwinSemantics(twin, lineages, workbench, telemetry);
validateFleetSemantics(fleetUnits, commercialBasis, workbench);
validateWorkbenchRefs(workbench);

if (problems.length) {
  console.error("Simulator Workbench contract check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log(
  `Simulator Workbench contract check passed: ${telemetry.length} measured frames, ${countTwinValues(twin)} twin values, ${fleetUnits.units?.length ?? 0} fleet units, ${workbench.panels?.length ?? 0} panels.`
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

function validateTwinSemantics(twin, lineages, workbench, telemetry) {
  const values = flattenTwinValues(twin);
  const selectedUnitId = workbench.selectedUnitId;
  const selectedValues = values.filter((value) => value.unitId === selectedUnitId);
  const valueBasisCounts = countByBasis(selectedValues);
  for (const basis of valueBasisValues) {
    if ((countByBasis(values)[basis] ?? 0) === 0) {
      problems.push(`twin state must include at least one ${basis} value across the mixed demo`);
    }
  }

  for (const lineage of lineages) {
    const lineagedValue = values.find((value) => value.valueId === lineage.valueId);
    if (!lineagedValue) {
      problems.push(`lineage ${lineage.lineageId} references missing twin value ${lineage.valueId}`);
    } else if (lineagedValue.valueBasis !== lineage.valueBasis) {
      problems.push(`lineage ${lineage.lineageId} basis ${lineage.valueBasis} does not match twin value ${lineagedValue.valueBasis}`);
    }

    const lineageInputBasis = new Set((lineage.inputs ?? []).map((input) => input.valueBasis));
    if (lineage.valueBasis === "imputed") {
      for (const basis of valueBasisValues) {
        if (!lineageInputBasis.has(basis)) {
          problems.push(`lineage ${lineage.lineageId} should show at least one ${basis} input for the mixed demo`);
        }
      }
    }
  }

  const expectedSummary = workbench.valueBasisSummary ?? {};
  for (const basis of valueBasisValues) {
    if (expectedSummary[basis] !== (valueBasisCounts[basis] ?? 0)) {
      problems.push(`workbench valueBasisSummary.${basis}=${expectedSummary[basis]} does not match selected-unit twin count ${valueBasisCounts[basis] ?? 0}`);
    }
  }

  validateCoreDistributionPrerequisites(values, telemetry);
}

function validateWorkbenchRefs(workbench) {
  for (const ref of [
    ...(workbench.measuredStateRefs ?? []),
    workbench.twinStateRef,
    ...(workbench.lineageRefs ?? []),
    ...(workbench.fleetUnitRefs ?? []),
    ...(workbench.commercialDisplayBasisRefs ?? [])
  ]) {
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

function validateCoreDistributionPrerequisites(values, telemetry) {
  const telemetryByTag = new Map(telemetry.map((frame) => [frame.tagId, frame]));
  const distributionValues = values.filter((value) => value.label === "Core Power Distribution Estimate");
  for (const value of distributionValues) {
    const fluxInputs = (value.sourceIds ?? []).filter((sourceId) => telemetryByTag.get(sourceId)?.signalKind === "flux");
    if (fluxInputs.length < 2) {
      problems.push(`${value.valueId}: Core Power Distribution Estimate requires multiple measured flux stand-ins`);
    }
    if (value.valueBasis !== "imputed") {
      problems.push(`${value.valueId}: Core Power Distribution Estimate must stay valueBasis=imputed`);
    }
  }
}

function validateFleetSemantics(fleetUnits, commercialBasis, workbench) {
  const units = fleetUnits.units ?? [];
  const basisRecords = commercialBasis.basis ?? [];
  if (units.length < 5) {
    problems.push(`fleet units example should include at least five units, got ${units.length}`);
  }

  const unitIds = new Set(units.map((unit) => unit.unitId));
  for (const expected of ["KAL-01", "KAL-02", "KAL-03", "KAL-04", "KAL-05"]) {
    if (!unitIds.has(expected)) {
      problems.push(`fleet units missing ${expected}`);
    }
  }

  const unplannedCount = units.filter((unit) => unit.availabilityPhase === "unplanned maintenance outage").length;
  if (unplannedCount > 1) {
    problems.push("at most one fixture unit may use unplanned maintenance outage");
  }

  for (const unit of units) {
    if (!allowedAvailabilityPhases.has(unit.availabilityPhase)) {
      problems.push(`${unit.unitId}: unsupported availability phase ${unit.availabilityPhase}`);
    }
    if (!allowedCommercialModes.has(unit.commercialMode)) {
      problems.push(`${unit.unitId}: unsupported commercial mode ${unit.commercialMode}`);
    }
    if (!basisRecords.some((basis) => basis.basisId === unit.commercialBasisId && basis.unitId === unit.unitId)) {
      problems.push(`${unit.unitId}: commercialBasisId ${unit.commercialBasisId} does not resolve for the same unit`);
    }
    if (unit.availabilityPhase === "cooldown" && unit.accruedDisplayValue?.compactLabel !== "no commercial output") {
      problems.push(`${unit.unitId}: cooldown unit must show no commercial output`);
    }
  }

  for (const basis of basisRecords) {
    if (!unitIds.has(basis.unitId)) {
      problems.push(`${basis.basisId}: commercial basis references unknown unit ${basis.unitId}`);
    }
    if (!allowedCommercialModes.has(basis.commercialMode)) {
      problems.push(`${basis.basisId}: unsupported commercial mode ${basis.commercialMode}`);
    }
    for (const exclusion of expectedCommercialExclusions) {
      if (!(basis.exclusions ?? []).includes(exclusion)) {
        problems.push(`${basis.basisId}: missing commercial display exclusion ${exclusion}`);
      }
    }
  }

  if (!unitIds.has(workbench.selectedUnitId)) {
    problems.push(`workbench selectedUnitId ${workbench.selectedUnitId} is not in fleet units`);
  }

  validateNoBannedCommercialTerms(fleetUnits, "fleet units");
  validateNoBannedCommercialTerms(commercialBasis, "commercial basis", new Set(["exclusions"]));
}

function flattenTwinValues(twin) {
  return (twin.entities ?? []).flatMap((entity) =>
    (entity.values ?? []).map((value) => ({
      ...value,
      unitId: entity.unitId,
      viewportEntity: entity.viewportEntity
    }))
  );
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

function validateNoBannedCommercialTerms(value, label, skipKeys = new Set()) {
  for (const found of collectStrings(value, skipKeys)) {
    const normalized = found.toLowerCase();
    for (const term of bannedCommercialTerms) {
      if (normalized.includes(term)) {
        problems.push(`${label}: banned commercial term ${term} appears in ${JSON.stringify(found)}`);
      }
    }
  }
}

function collectStrings(value, skipKeys) {
  if (typeof value === "string") {
    return [value];
  }
  if (Array.isArray(value)) {
    return value.flatMap((item) => collectStrings(item, skipKeys));
  }
  if (isPlainObject(value)) {
    return Object.entries(value).flatMap(([key, child]) => (skipKeys.has(key) ? [] : collectStrings(child, skipKeys)));
  }
  return [];
}
