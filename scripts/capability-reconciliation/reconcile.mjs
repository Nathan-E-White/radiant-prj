import { createHash } from "node:crypto";

const classifications = new Set(["active", "intentional-retirement", "superseded-capability", "incomplete-migration", "accidental-regression", "insufficient-evidence"]);

export async function reconcileCapabilities({ historicalLedger, historicalRevision, currentLedger, verifyCapability, manifest, root, repositoryVerification } = {}) {
  const historical = historicalLedger?.capabilities;
  if (!Array.isArray(historical)) return report(historicalRevision, [insufficient("ledger", "historical-ledger", `historical ledger at ${historicalRevision} has no capabilities array`)]);
  const current = Array.isArray(currentLedger?.capabilities) ? currentLedger.capabilities : [];
  const currentById = new Map(current.map((capability) => [capability.id, capability]));
  const findings = [];
  for (const commitment of [...historical].sort((left, right) => left.id.localeCompare(right.id))) {
    const commitmentRevision = commitment.historicalRevision ?? historicalRevision;
    const present = currentById.get(commitment.id);
    const successor = present?.successorId ? currentById.get(present.successorId) : undefined;
    const evidence = await currentEvidence({ present, successor, currentLedger, manifest, root, verifyCapability, repositoryVerification });
    const classification = classify({ commitment, present, successor, evidence });
    findings.push(finding({ historicalRevision: commitmentRevision, commitment, present, successor, classification, evidence }));
  }
  return report(historicalRevision, findings);
}

function classify({ commitment, present, successor, evidence }) {
  if (!validCommitment(commitment)) return "insufficient-evidence";
  if (!present) return "accidental-regression";
  if (present.lifecycle === "intentionally-retired") return "intentional-retirement";
  if (present.lifecycle === "under-reconciliation") return "incomplete-migration";
  if (present.lifecycle === "superseded") return successor?.lifecycle === "active" && successorEvidencePasses(evidence) ? "superseded-capability" : "incomplete-migration";
  if (present.lifecycle !== "active") return "insufficient-evidence";
  return evidence?.status === "passed" ? "active" : evidence?.status === "failed" ? "accidental-regression" : "insufficient-evidence";
}

async function currentEvidence({ present, successor, currentLedger, manifest, root, verifyCapability, repositoryVerification }) {
  const target = successor ?? present;
  if (!target) return { status: "missing", claim: null, detail: "no mapped current capability" };
  if (!verifyCapability || !manifest || !currentLedger) return { status: "unavailable", claim: target.verificationClaim, detail: "Repository Verification adapter was not supplied" };
  try {
    const result = await verifyCapability({ ledger: currentLedger, manifest, capabilityId: target.id, root, ...(repositoryVerification ? { run: repositoryVerification } : {}) });
    return { status: result.exitCode === 0 ? "passed" : "failed", claim: target.verificationClaim, report: result };
  } catch (error) {
    return { status: "failed", claim: target.verificationClaim, detail: error.message };
  }
}

function successorEvidencePasses(evidence) { return evidence?.status === "passed"; }
function validCommitment(commitment) { return commitment && typeof commitment.id === "string" && commitment.id.trim() !== "" && typeof commitment.verificationClaim === "string" && commitment.verificationClaim.trim() !== ""; }
function insufficient(id, claim, detail) { return finding({ historicalRevision: "unavailable", commitment: { id, verificationClaim: claim }, classification: "insufficient-evidence", evidence: { status: "unavailable", claim, detail } }); }

function finding({ historicalRevision, commitment, present, successor, classification, evidence }) {
  if (!classifications.has(classification)) throw new Error(`unsupported reconciliation classification: ${classification}`);
  const historicalId = commitment.id;
  const mappedId = successor?.id ?? present?.id ?? null;
  return {
    id: stableId(historicalRevision, historicalId),
    classification,
    historicalCommitment: { revision: historicalRevision, capabilityId: historicalId, verificationClaim: commitment.verificationClaim, evidence: "config/capability-ledger.json" },
    presentEvidence: mappedId ? { capabilityId: mappedId, lifecycle: successor?.lifecycle ?? present.lifecycle, verificationClaim: successor?.verificationClaim ?? present.verificationClaim, repositoryVerification: evidence } : { repositoryVerification: evidence },
    requiredClassification: classification
  };
}

function report(historicalRevision, findings) {
  return { schemaVersion: "radiant.capability-reconciliation.v1", historicalRevision, findings: findings.sort((left, right) => left.id.localeCompare(right.id)) };
}
function stableId(revision, capabilityId) { return `reconciliation-${createHash("sha256").update(`${revision}:${capabilityId}`).digest("hex").slice(0, 16)}`; }
