import { expect, test } from "vitest";

import { affectedCapabilities, validateLedger, verifyCapability } from "./capability-ledger/ledger.mjs";

const manifest = {
  schemaVersion: "radiant.repository-verification.v1",
  claims: [{ id: "test.claim", title: "test", expected: "pass", evidence: { adapter: "command", source: "test", command: ["test"] } }]
};
const record = {
  id: "capability", title: "Capability", lifecycle: "active", ownerRole: "Software", decisionRef: "issue-175",
  documentationRefs: ["docs/verification/verification-plan.md"],
  surfaces: [{ kind: "source-set", root: "scripts/repository-verification", extensions: [".mjs"] }, { kind: "artifact", path: "package.json" }],
  verificationClaim: "test.claim", operationalConstraints: ["bounded"], lastVerificationEvidence: { revision: "baseline", reference: "test.claim" }
};
const ledger = { schemaVersion: "radiant.capability-ledger.v1", capabilities: [record] };

test("accepts a valid record and maps only active matching source-set paths", () => {
  expect(validateLedger(ledger, { manifest, root: process.cwd() })).toEqual([]);
  expect(affectedCapabilities(ledger, ["scripts/repository-verification/moved.mjs"])).toEqual(["capability"]);
  expect(affectedCapabilities(ledger, ["scripts/repository-verification/moved.ts", "README.md"])).toEqual([]);
});

test("enforces lifecycle, claim, successor, document, surface, and evidence invariants", () => {
  const invalid = structuredClone(ledger);
  Object.assign(invalid.capabilities[0], { lifecycle: "bad", verificationClaim: "missing", successorId: "capability" });
  invalid.capabilities[0].documentationRefs = ["missing.md"];
  invalid.capabilities[0].surfaces = [{ kind: "bad", path: "missing" }];
  invalid.capabilities[0].lastVerificationEvidence = {};
  const problems = validateLedger(invalid, { manifest, root: process.cwd() }).join("\n");
  expect(problems).toContain("lifecycle");
  expect(problems).toContain("missing claim");
  expect(problems).toContain("must not reference itself");
  expect(problems).toContain("missing controlled document");
  expect(problems).toContain("kind must be artifact or source-set");
  expect(problems).toContain("lastVerificationEvidence");
});

test("rejects invalid top-level shapes, required fields, duplicate IDs, and invalid source mappings", () => {
  expect(validateLedger(null, { manifest, root: process.cwd() })).toEqual(["ledger must be an object"]);
  expect(validateLedger({ schemaVersion: "wrong", capabilities: {} }, { manifest, root: process.cwd() })).toEqual(["schemaVersion must equal radiant.capability-ledger.v1", "capabilities must be an array"]);
  const invalid = structuredClone(ledger);
  invalid.capabilities[0] = {
    ...invalid.capabilities[0], id: "", title: "", ownerRole: "", decisionRef: "", verificationClaim: "", documentationRefs: [], operationalConstraints: "bad",
    surfaces: [{ kind: "artifact", path: "missing" }, { kind: "source-set", root: "missing", extensions: [""] }]
  };
  invalid.capabilities.push({ ...structuredClone(record), id: "" });
  const problems = validateLedger(invalid, { manifest, root: process.cwd() }).join("\n");
  for (const wording of ["id must be", "title must be", "ownerRole must be", "decisionRef must be", "verificationClaim must be", "documentationRefs must", "operationalConstraints must", "path is missing", "root is missing", "extensions must"]) expect(problems).toContain(wording);
});

test("maps artifacts exactly, normalizes paths, ignores inactive records, and validates successor targets", () => {
  const records = structuredClone(ledger);
  records.capabilities[0].surfaces = [{ kind: "artifact", path: "package.json" }];
  expect(affectedCapabilities(records, ["./package.json"])).toEqual(["capability"]);
  records.capabilities[0].lifecycle = "intentionally-retired";
  expect(affectedCapabilities(records, ["package.json"])).toEqual([]);
  records.capabilities[0].lifecycle = "under-reconciliation";
  expect(affectedCapabilities(records, ["package.json"])).toEqual([]);
  records.capabilities[0].lifecycle = "active";
  records.capabilities[0].successorId = "missing";
  expect(validateLedger(records, { manifest, root: process.cwd() }).join("\n")).toContain("references missing capability missing");
});

test("preserves all accepted lifecycle values and normalizes strict source-set matching", () => {
  for (const lifecycle of ["active", "superseded", "intentionally-retired", "under-reconciliation"]) {
    const candidate = structuredClone(ledger);
    candidate.capabilities[0].lifecycle = lifecycle;
    if (lifecycle === "superseded") {
      candidate.capabilities[0].successorId = "successor";
      candidate.capabilities.push({ ...structuredClone(record), id: "successor" });
    }
    expect(validateLedger(candidate, { manifest, root: process.cwd() })).toEqual([]);
  }
  const mapped = structuredClone(ledger);
  mapped.capabilities[0].surfaces = [{ kind: "source-set", root: "scripts/repository-verification", extensions: [".mjs", ".json"] }];
  expect(affectedCapabilities(mapped, ["scripts\\repository-verification\\cli.mjs"])).toEqual(["capability"]);
  expect(affectedCapabilities(mapped, ["scripts/repository-verification/config.json"])).toEqual(["capability"]);
  expect(affectedCapabilities(mapped, ["scripts/repository-verification2/cli.mjs", "scripts/repository-verification/cli.ts"])).toEqual([]);
});

test("treats whitespace as malformed and accepts an extension-free source set", () => {
  const whitespace = structuredClone(ledger);
  whitespace.capabilities[0].title = "   ";
  whitespace.capabilities[0].lastVerificationEvidence.reference = " ";
  expect(validateLedger(whitespace, { manifest, root: process.cwd() }).join("\n")).toContain("must be a non-empty string");
  const extensionFree = structuredClone(ledger);
  extensionFree.capabilities[0].surfaces = [{ kind: "source-set", root: "scripts/repository-verification" }];
  expect(affectedCapabilities(extensionFree, ["scripts/repository-verification/any.extension"])).toEqual(["capability"]);
});

test("rejects duplicate capability IDs, unknown claims, and invalid surface extension shapes", () => {
  const invalid = structuredClone(ledger);
  invalid.capabilities.push({ ...structuredClone(record) });
  invalid.capabilities[0].verificationClaim = "unknown.claim";
  invalid.capabilities[0].surfaces = [{ kind: "source-set", root: "scripts/repository-verification", extensions: "mjs" }];
  const problems = validateLedger(invalid, { manifest, root: process.cwd() }).join("\n");
  expect(problems).toContain("duplicates capability");
  expect(problems).toContain("references missing claim unknown.claim");
  expect(problems).toContain("extensions must be an array of strings");
});

test("does not mistake an artifact prefix or a directory itself for a mapped artifact", () => {
  const artifact = structuredClone(ledger);
  artifact.capabilities[0].surfaces = [{ kind: "artifact", path: "package.json" }];
  expect(affectedCapabilities(artifact, ["package.json.bak", "package"])).toEqual([]);
  const sourceSet = structuredClone(ledger);
  sourceSet.capabilities[0].surfaces = [{ kind: "source-set", root: "scripts/repository-verification", extensions: [".mjs"] }];
  expect(affectedCapabilities(sourceSet, ["scripts/repository-verification"])).toEqual([]);
});

test("handles empty extension lists, duplicate changed paths, and normalized artifact paths", () => {
  const sourceSet = structuredClone(ledger);
  sourceSet.capabilities[0].surfaces = [{ kind: "source-set", root: "scripts/repository-verification", extensions: [] }];
  expect(affectedCapabilities(sourceSet, ["./scripts/repository-verification/one.ts", "scripts/repository-verification/one.ts"])).toEqual(["capability"]);
  const artifact = structuredClone(ledger);
  artifact.capabilities[0].surfaces = [{ kind: "artifact", path: "package.json" }];
  expect(affectedCapabilities(artifact, [".\\package.json"])).toEqual(["capability"]);
});

test("requires every record field that the public ledger contract promises", () => {
  for (const field of ["id", "title", "lifecycle", "ownerRole", "decisionRef", "verificationClaim"]) {
    const invalid = structuredClone(ledger);
    delete invalid.capabilities[0][field];
    expect(validateLedger(invalid, { manifest, root: process.cwd() }).join("\n")).toContain(`${field} must be a non-empty string`);
  }
});

test("requires a valid successor and delegates named verification through the supplied runner", async () => {
  const superseded = structuredClone(ledger);
  superseded.capabilities[0].lifecycle = "superseded";
  expect(validateLedger(superseded, { manifest, root: process.cwd() }).join("\n")).toContain("successorId is required");
  superseded.capabilities[0].successorId = "successor";
  superseded.capabilities.push({ ...structuredClone(record), id: "successor" });
  expect(validateLedger(superseded, { manifest, root: process.cwd() })).toEqual([]);
  const report = await verifyCapability({ ledger, manifest, capabilityId: "capability", root: process.cwd(), run: () => ({ status: 1, stdout: "", stderr: "fault" }) });
  expect(report.exitCode).toBe(1);
  expect(report.results[0].status).toBe("fail");
  await expect(verifyCapability({ ledger, manifest, capabilityId: "missing", root: process.cwd() })).rejects.toThrow("unknown capability");
});

test("rejects cyclic successor relationships", () => {
  const cyclic = structuredClone(ledger);
  cyclic.capabilities[0].lifecycle = "superseded";
  cyclic.capabilities[0].successorId = "next";
  cyclic.capabilities.push({ ...structuredClone(record), id: "next", lifecycle: "superseded", successorId: "capability" });
  expect(validateLedger(cyclic, { manifest, root: process.cwd() }).join("\n")).toContain("successor cycle");
});
