import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const manifest = JSON.parse(readFileSync("config/repository-verification.json", "utf8"));

test("repository manifest registers the Workbench Snapshot smoke contract as executable evidence", () => {
  const claim = manifest.claims.find(({ id }) => id === "simulator-workbench.snapshot-smoke-contract");

  assert.equal(claim?.evidence?.adapter, "command");
  assert.deepEqual(claim?.evidence?.command, ["node", "--test", "scripts/workbench-dataflow-json.node-test.mjs"]);
  assert.match(claim?.expected ?? "", /complete generation-matched Snapshot/);
  assert.match(claim?.expected ?? "", /partial or generation-mismatched/);
});

test("repository manifest inventories the accepted Snapshot and lifecycle decision records", () => {
  const claim = manifest.claims.find(({ id }) => id === "capability-lifecycle.documentation-inventory");
  const sources = claim?.evidence?.sources ?? [];

  assert.ok(sources.includes("docs/adr/adr-0013.md"));
  assert.ok(sources.includes("docs/design/workbench-snapshot-read-and-lifecycle-policy.md"));
});
