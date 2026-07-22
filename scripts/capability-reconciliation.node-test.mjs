import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { loadHistoricalLedger, loadHistoricalRange } from "./capability-reconciliation/git-adapter.mjs";
import { reconcileCapabilities } from "./capability-reconciliation/reconcile.mjs";

const historical = { capabilities: [{ id: "introduced-then-removed", verificationClaim: "historical.claim" }] };
const active = { capabilities: [{ id: "introduced-then-removed", lifecycle: "active", verificationClaim: "current.claim" }] };
const pass = async () => ({ exitCode: 0, results: [{ claimId: "current.claim" }] });
const fail = async () => ({ exitCode: 1, results: [{ claimId: "current.claim" }] });

async function reconcile(currentLedger, verifier = pass) { return reconcileCapabilities({ historicalLedger: historical, historicalRevision: "abc123", currentLedger, manifest: { claims: [] }, verifyCapability: verifier }); }
function only(report) { return report.findings[0]; }

test("classifies active capability with Repository Verification evidence", async () => {
  const finding = only(await reconcile(active));
  assert.equal(finding.classification, "active");
  assert.equal(finding.presentEvidence.repositoryVerification.status, "passed");
  assert.match(finding.id, /^reconciliation-/);
});
test("classifies an explicit retirement without silently treating absence as retirement", async () => {
  assert.equal(only(await reconcile({ capabilities: [{ ...active.capabilities[0], lifecycle: "intentionally-retired" }] })).classification, "intentional-retirement");
  assert.equal(only(await reconcile({ capabilities: [] })).classification, "accidental-regression");
});
test("classifies superseded capability only with passing successor evidence", async () => {
  const ledger = { capabilities: [{ ...active.capabilities[0], lifecycle: "superseded", successorId: "replacement" }, { id: "replacement", lifecycle: "active", verificationClaim: "replacement.claim" }] };
  assert.equal(only(await reconcile(ledger)).classification, "superseded-capability");
  assert.equal(only(await reconcile(ledger, fail)).classification, "incomplete-migration");
});
test("classifies under-reconciliation and failed active evidence", async () => {
  assert.equal(only(await reconcile({ capabilities: [{ ...active.capabilities[0], lifecycle: "under-reconciliation" }] })).classification, "incomplete-migration");
  assert.equal(only(await reconcile(active, fail)).classification, "accidental-regression");
});
test("classifies malformed historical commitment as insufficient evidence", async () => {
  const report = await reconcileCapabilities({ historicalLedger: { capabilities: [{ id: "missing-claim" }] }, historicalRevision: "abc123", currentLedger: active, manifest: { claims: [] }, verifyCapability: pass });
  assert.equal(only(report).classification, "insufficient-evidence");
});
test("orders findings and identifiers deterministically", async () => {
  const report = await reconcileCapabilities({ historicalLedger: { capabilities: [{ id: "z", verificationClaim: "z" }, { id: "a", verificationClaim: "a" }] }, historicalRevision: "abc123", currentLedger: { capabilities: [] }, manifest: { claims: [] }, verifyCapability: pass });
  assert.deepEqual(report.findings.map((finding) => finding.historicalCommitment.capabilityId), ["a", "z"]);
  assert.deepEqual(report, await reconcileCapabilities({ historicalLedger: { capabilities: [{ id: "z", verificationClaim: "z" }, { id: "a", verificationClaim: "a" }] }, historicalRevision: "abc123", currentLedger: { capabilities: [] }, manifest: { claims: [] }, verifyCapability: pass }));
});
test("Git adapter reads a selected local baseline without network access", () => {
  const calls = [];
  const result = loadHistoricalLedger({ baseline: "v1", root: "/repo", run: (command, args) => { calls.push([command, args]); return args[0] === "rev-parse" ? "abc123\n" : JSON.stringify(historical); } });
  assert.equal(result.revision, "abc123");
  assert.deepEqual(result.ledger, historical);
  assert.deepEqual(calls[1], ["git", ["show", "abc123:config/capability-ledger.json"]]);
});
test("Git adapter preserves commitments introduced anywhere in a selected range", () => {
  const result = loadHistoricalRange({ range: "start..end", root: "/repo", run: (_command, args) => {
    if (args[0] === "rev-list") return "one\ntwo\n";
    if (args[1].startsWith("one:")) return JSON.stringify({ capabilities: [{ id: "introduced-then-removed", verificationClaim: "old.claim" }] });
    return JSON.stringify({ capabilities: [] });
  } });
  assert.equal(result.ledger.capabilities[0].id, "introduced-then-removed");
  assert.equal(result.ledger.capabilities[0].historicalRevision, "one");
});
test("CI records reconciliation evidence without issue-tracker access", () => {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  assert.match(workflow, /run: bun run capability:reconcile -- HEAD/);
  assert.match(workflow, /name: capability-reconciliation-evidence/);
  assert.doesNotMatch(workflow, /capability-reconciliation[\s\S]*github\.com\/.*issues/);
});
