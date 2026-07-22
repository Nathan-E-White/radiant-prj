import { existsSync } from "node:fs";
import path from "node:path";
import { verifyRepository } from "../repository-verification/verifier.mjs";

const schemaVersion = "radiant.capability-ledger.v1";
const lifecycleStates = new Set(["active", "superseded", "intentionally-retired", "under-reconciliation"]);

export function validateLedger(ledger, { manifest, root = process.cwd() } = {}) {
  const problems = [];
  if (!ledger || typeof ledger !== "object") return ["ledger must be an object"];
  if (ledger.schemaVersion !== schemaVersion) problems.push(`schemaVersion must equal ${schemaVersion}`);
  if (!Array.isArray(ledger.capabilities)) return [...problems, "capabilities must be an array"];
  const ids = new Set();
  const claimIds = new Set((manifest?.claims ?? []).map(({ id }) => id));
  for (const [index, capability] of ledger.capabilities.entries()) {
    const prefix = `capabilities[${index}]`;
    for (const field of ["id", "title", "lifecycle", "ownerRole", "decisionRef", "verificationClaim"]) if (!nonEmpty(capability?.[field])) problems.push(`${prefix}.${field} must be a non-empty string`);
    if (ids.has(capability?.id)) problems.push(`${prefix}.id duplicates ${capability.id}`);
    ids.add(capability?.id);
    if (!lifecycleStates.has(capability?.lifecycle)) problems.push(`${prefix}.lifecycle must be one of ${[...lifecycleStates].join(", ")}`);
    if (!claimIds.has(capability?.verificationClaim)) problems.push(`${prefix}.verificationClaim references missing claim ${capability?.verificationClaim}`);
    validateReferences(capability?.documentationRefs, prefix, root, problems);
    validateSurfaces(capability?.surfaces, prefix, root, problems);
    if (!Array.isArray(capability?.operationalConstraints)) problems.push(`${prefix}.operationalConstraints must be an array`);
    if (!nonEmpty(capability?.lastVerificationEvidence?.revision) || !nonEmpty(capability?.lastVerificationEvidence?.reference)) problems.push(`${prefix}.lastVerificationEvidence requires revision and reference`);
    if (capability?.lifecycle === "superseded" && !nonEmpty(capability.successorId)) problems.push(`${prefix}.successorId is required for superseded capability`);
    if (capability?.successorId === capability?.id) problems.push(`${prefix}.successorId must not reference itself`);
  }
  const capabilitiesById = new Map(ledger.capabilities.map((capability) => [capability?.id, capability]));
  for (const [index, capability] of ledger.capabilities.entries()) {
    if (capability?.successorId && !ids.has(capability.successorId)) problems.push(`capabilities[${index}].successorId references missing capability ${capability.successorId}`);
    const visited = new Set([capability?.id]);
    let successorId = capability?.successorId;
    while (successorId) {
      if (visited.has(successorId)) {
        problems.push(`capabilities[${index}].successorId creates a successor cycle at ${successorId}`);
        break;
      }
      visited.add(successorId);
      successorId = capabilitiesById.get(successorId)?.successorId;
    }
  }
  return problems.sort();
}

export async function verifyCapability({ ledger, manifest, capabilityId, root = process.cwd(), run } = {}) {
  const problems = validateLedger(ledger, { manifest, root });
  if (problems.length) throw new Error(`Ledger validation failed:\n- ${problems.join("\n- ")}`);
  const capability = ledger.capabilities.find(({ id }) => id === capabilityId);
  if (!capability) throw new Error(`unknown capability: ${capabilityId}`);
  return verifyRepository({ manifest, root, claimIds: [capability.verificationClaim], ...(run ? { run } : {}) });
}

export function affectedCapabilities(ledger, changedPaths) {
  const paths = [...new Set(changedPaths.map(normalize))];
  return (ledger?.capabilities ?? []).filter(({ lifecycle, surfaces = [] }) => lifecycle === "active" && surfaces.some((surface) => paths.some((changedPath) => matches(surface, changedPath)))).map(({ id }) => id).sort();
}

function validateReferences(references, prefix, root, problems) {
  if (!Array.isArray(references) || references.length === 0) return problems.push(`${prefix}.documentationRefs must be a non-empty array`);
  references.forEach((reference) => { if (!nonEmpty(reference) || !existsSync(path.resolve(root, reference))) problems.push(`${prefix}.documentationRefs contains missing controlled document ${reference}`); });
}

function validateSurfaces(surfaces, prefix, root, problems) {
  if (!Array.isArray(surfaces) || surfaces.length === 0) return problems.push(`${prefix}.surfaces must be a non-empty array`);
  surfaces.forEach((surface, index) => {
    const item = `${prefix}.surfaces[${index}]`;
    if (!surface || !["artifact", "source-set"].includes(surface.kind)) problems.push(`${item}.kind must be artifact or source-set`);
    if (!nonEmpty(surface?.path ?? surface?.root)) problems.push(`${item} requires path or root`);
    if (surface?.kind === "artifact" && !existsSync(path.resolve(root, surface.path))) problems.push(`${item}.path is missing: ${surface.path}`);
    if (surface?.kind === "source-set" && !existsSync(path.resolve(root, surface.root))) problems.push(`${item}.root is missing: ${surface.root}`);
    if (surface?.extensions && (!Array.isArray(surface.extensions) || surface.extensions.some((extension) => !nonEmpty(extension)))) problems.push(`${item}.extensions must be an array of strings`);
  });
}

function matches(surface, changedPath) {
  if (surface.kind === "artifact") return normalize(surface.path) === changedPath;
  const root = normalize(surface.root).replace(/\/$/, "");
  return surface.kind === "source-set" && (changedPath === root || changedPath.startsWith(`${root}/`)) && (!surface.extensions?.length || surface.extensions.some((extension) => changedPath.endsWith(extension)));
}
function normalize(value) { return String(value).replace(/\\/g, "/").replace(/^\.\//, ""); }
function nonEmpty(value) { return typeof value === "string" && value.trim() !== ""; }
