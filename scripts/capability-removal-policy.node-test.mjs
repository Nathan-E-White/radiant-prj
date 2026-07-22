import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { evaluateCapabilityRemovalPolicy } from "./capability-removal-policy/policy.mjs";

const capability = { id: "delivery", lifecycle: "active", verificationClaim: "delivery.claim", surfaces: [{ kind: "artifact", path: ".github/workflows/delivery.yml" }, { kind: "source-set", root: "scripts/delivery", extensions: [".mjs"] }] };
const manifest = { claims: [{ id: "delivery.claim" }, { id: "replacement.claim" }] };
const change = [{ status: "D", path: ".github/workflows/delivery.yml" }];
const evaluate = ({ currentLedger = { capabilities: [capability] }, currentManifest = manifest, verifiedClaimIds, evidence = { declarations: [] }, changes = change } = {}) => evaluateCapabilityRemovalPolicy({ previousLedger: { capabilities: [capability] }, currentLedger, currentManifest, verifiedClaimIds, evidence, changes });

test("fails an undeclared mapped artifact deletion actionably", () => {
  const report = evaluate();
  assert.equal(report.exitCode, 1);
  assert.deepEqual(report.affectedCapabilities, ["delivery"]);
  assert.match(report.findings[0].required, /declare preserve/);
});

test("permits explicit preservation for a mapped artifact move", () => {
  const report = evaluate({
    changes: [{ status: "R", oldPath: ".github/workflows/delivery.yml", path: ".github/workflows/delivery-v2.yml" }],
    currentLedger: { capabilities: [{ ...capability, surfaces: [{ kind: "artifact", path: ".github/workflows/delivery-v2.yml" }] }] },
    evidence: { declarations: [{ capabilityId: "delivery", action: "preserve", removed: { kind: "artifact", path: ".github/workflows/delivery.yml" } }] }
  });
  assert.equal(report.exitCode, 0);
  assert.equal(report.findings[0].action, "preserve");
});

test("gates a removed verification claim independently of its mapped artifact", () => {
  const report = evaluate({ currentManifest: { claims: [] }, changes: [] });
  assert.equal(report.exitCode, 1);
  assert.deepEqual(report.findings[0], {
    status: "fail", capabilityId: "delivery", kind: "verification-claim", id: "delivery.claim",
    required: "declare preserve, retire, or supersede evidence for this removed contract"
  });
});

test("permits retirement and verified supersession, but rejects a missing successor claim", () => {
  const evidence = { declarations: [{ capabilityId: "delivery", action: "retire", removed: { kind: "artifact", path: ".github/workflows/delivery.yml" } }] };
  assert.equal(evaluate({ currentLedger: { capabilities: [{ ...capability, lifecycle: "intentionally-retired" }] }, evidence }).exitCode, 0);
  const supersede = { declarations: [{ capabilityId: "delivery", action: "supersede", removed: { kind: "artifact", path: ".github/workflows/delivery.yml" } }] };
  const successor = { ...capability, id: "delivery-v2", verificationClaim: "replacement.claim" };
  const currentLedger = { capabilities: [{ ...capability, lifecycle: "superseded", successorId: successor.id }, successor] };
  assert.equal(evaluate({ currentLedger, evidence: supersede, verifiedClaimIds: ["replacement.claim"] }).exitCode, 0);
  assert.equal(evaluate({ currentLedger, evidence: supersede, verifiedClaimIds: [] }).exitCode, 1);
  assert.equal(evaluate({ currentLedger, currentManifest: { claims: [] }, evidence: supersede }).exitCode, 1);
});

test("does not gate unrelated changes or a source-set internal refactor", () => {
  assert.equal(evaluate({ changes: [{ status: "M", path: "README.md" }] }).exitCode, 0);
  assert.equal(evaluate({ changes: [{ status: "D", path: "scripts/delivery/moved-internally.mjs" }] }).exitCode, 0);
});

test("CI invokes the same change-impact command as local users", () => {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  assert.match(workflow, /bun run capability:removal:check -- \$\{\{ github.event.pull_request.base.sha \|\| github.event.before \}\}/);
  assert.match(workflow, /if: github.event_name == 'pull_request' \|\| github.event.before != '0000000000000000000000000000000000000000'/);
});
