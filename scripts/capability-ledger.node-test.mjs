import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { affectedCapabilities, validateLedger, verifyCapability } from "./capability-ledger/ledger.mjs";

const manifest = {
  schemaVersion: "radiant.repository-verification.v1",
  claims: [{ id: "browser.delivery", title: "Browser delivery", expected: "passes", evidence: { adapter: "files", source: "package.json", sources: ["package.json"] } }]
};

const ledger = {
  schemaVersion: "radiant.capability-ledger.v1",
  capabilities: [{
    id: "browser-delivery",
    title: "Browser delivery",
    lifecycle: "active",
    ownerRole: "Software",
    decisionRef: "https://github.com/Nathan-E-White/radiant-prj/issues/139",
    documentationRefs: ["docs/verification/verification-plan.md"],
    surfaces: [{ kind: "source-set", root: "scripts/repository-verification", extensions: [".mjs"] }],
    verificationClaim: "browser.delivery",
    operationalConstraints: ["local and CI use the same claim"],
    lastVerificationEvidence: { revision: "controlled-baseline", reference: "browser.delivery" }
  }]
};

test("validates a retained active capability", () => {
  assert.deepEqual(validateLedger(ledger, { manifest, root: process.cwd() }), []);
});

test("reports malformed records and missing claim references", () => {
  const invalid = structuredClone(ledger);
  invalid.capabilities[0].lifecycle = "unknown";
  invalid.capabilities[0].verificationClaim = "missing.claim";
  const problems = validateLedger(invalid, { manifest, root: process.cwd() });
  assert.ok(problems.some((problem) => problem.includes("lifecycle")));
  assert.ok(problems.some((problem) => problem.includes("missing.claim")));
});

test("requires a successor for a superseded capability", () => {
  const invalid = structuredClone(ledger);
  invalid.capabilities[0].lifecycle = "superseded";
  assert.ok(validateLedger(invalid, { manifest, root: process.cwd() }).some((problem) => problem.includes("successorId")));
});

test("retains historical surface mappings after an intentional retirement", () => {
  const retired = structuredClone(ledger);
  retired.capabilities[0].lifecycle = "intentionally-retired";
  retired.capabilities[0].surfaces = [{ kind: "artifact", path: "removed-delivery-workflow.yml" }];
  assert.deepEqual(validateLedger(retired, { manifest, root: process.cwd() }), []);
});

test("accepts a superseded capability only when its successor exists", () => {
  const superseded = structuredClone(ledger);
  superseded.capabilities[0].lifecycle = "superseded";
  superseded.capabilities[0].successorId = "browser-delivery-next";
  superseded.capabilities.push({ ...structuredClone(ledger.capabilities[0]), id: "browser-delivery-next", title: "Browser delivery successor" });
  assert.deepEqual(validateLedger(superseded, { manifest, root: process.cwd() }), []);
});

test("rejects cyclic successor chains deterministically", () => {
  const cyclic = structuredClone(ledger);
  cyclic.capabilities[0].lifecycle = "superseded";
  cyclic.capabilities[0].successorId = "successor";
  cyclic.capabilities.push({ ...structuredClone(ledger.capabilities[0]), id: "successor", lifecycle: "superseded", successorId: "browser-delivery" });
  assert.ok(validateLedger(cyclic, { manifest, root: process.cwd() }).some((problem) => problem.includes("successor cycle")));
});

test("finds active capabilities affected by a source-set path without pinning an internal filename", () => {
  assert.deepEqual(affectedCapabilities(ledger, ["scripts/repository-verification/moved-internally.mjs"]), ["browser-delivery"]);
  assert.deepEqual(affectedCapabilities(ledger, ["docs/verification/verification-plan.md"]), []);
});

test("returns affected capability identifiers in deterministic order and ignores unrelated paths", () => {
  const two = structuredClone(ledger);
  two.capabilities.push({ ...structuredClone(ledger.capabilities[0]), id: "another-browser-delivery" });
  assert.deepEqual(affectedCapabilities(two, ["scripts/repository-verification/moved.mjs"]), ["another-browser-delivery", "browser-delivery"]);
  assert.deepEqual(affectedCapabilities(two, ["README.md"]), []);
});

test("verifies one named capability through the Repository Verification seam", async () => {
  const report = await verifyCapability({ ledger, manifest, capabilityId: "browser-delivery", root: process.cwd() });
  assert.equal(report.exitCode, 0);
  assert.equal(report.results[0].claimId, "browser.delivery");
});

test("reports an injected claim-execution fault and recovers on the next verification", async () => {
  const commandManifest = structuredClone(manifest);
  commandManifest.claims[0].evidence = { adapter: "command", source: "bounded test command", command: ["bounded-test"] };
  const failed = await verifyCapability({
    ledger,
    manifest: commandManifest,
    capabilityId: "browser-delivery",
    root: process.cwd(),
    run: () => ({ status: 1, stdout: "", stderr: "bounded injected fault" })
  });
  assert.equal(failed.exitCode, 1);
  assert.match(failed.results[0].observed, /bounded injected fault/);
  const recovered = await verifyCapability({ ledger, manifest: commandManifest, capabilityId: "browser-delivery", root: process.cwd(), run: () => ({ status: 0, stdout: "recovered", stderr: "" }) });
  assert.equal(recovered.exitCode, 0);
});

test("fails unknown named-capability lookup actionably", async () => {
  await assert.rejects(
    verifyCapability({ ledger, manifest, capabilityId: "missing", root: process.cwd() }),
    /unknown capability: missing/
  );
});

test("CI invokes the same Ledger validation command as local users", () => {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  assert.match(workflow, /run: bun run capability:ledger:check/);
});
