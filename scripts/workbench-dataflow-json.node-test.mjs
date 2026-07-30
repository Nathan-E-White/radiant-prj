import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const script = new URL("./workbench-dataflow-json.mjs", import.meta.url);

test("snapshot-ready accepts one coherent Workbench Snapshot", () => {
  const result = runHelper(completeSnapshot());

  assert.equal(result.status, 0, result.stderr);
});

test("snapshot-ready rejects a generation mismatch", () => {
  const result = runHelper(vector("workbench-snapshot.generation-mismatch.json"));

  assert.notEqual(result.status, 0);
});

test("snapshot-ready rejects a partial Snapshot", () => {
  const result = runHelper(vector("workbench-snapshot.partial.json"));

  assert.notEqual(result.status, 0);
});

function runHelper(snapshot) {
  return spawnSync(process.execPath, [script.pathname, "snapshot-ready"], {
    input: JSON.stringify(snapshot),
    encoding: "utf8",
  });
}

function completeSnapshot() {
  return vector("workbench-snapshot.valid.json");
}

function vector(name) { return JSON.parse(readFileSync(new URL(`../examples/simulator-workbench/${name}`, import.meta.url), "utf8")); }
