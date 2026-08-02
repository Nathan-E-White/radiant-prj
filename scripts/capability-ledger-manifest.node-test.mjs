import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { validateLedger } from "./capability-ledger/ledger.mjs";

const manifest = JSON.parse(readFileSync("config/repository-verification.json", "utf8"));
const ledger = JSON.parse(readFileSync("config/capability-ledger.json", "utf8"));

test("Snapshot smoke integrity capability is valid and bound to its repository claim", () => {
  assert.deepEqual(validateLedger(ledger, { manifest, root: process.cwd() }), []);

  const capability = ledger.capabilities.find(({ id }) => id === "simulator-workbench-snapshot-smoke-contract");
  assert.equal(capability?.lifecycle, "active");
  assert.equal(capability?.verificationClaim, "simulator-workbench.snapshot-smoke-contract");
  assert.ok(capability?.documentationRefs.includes("docs/adr/adr-0013.md"));
  assert.ok(capability?.documentationRefs.includes("docs/design/workbench-snapshot-read-and-lifecycle-policy.md"));
  assert.ok(capability?.surfaces.some(({ path }) => path === "scripts/workbench-dataflow-json.node-test.mjs"));
});
