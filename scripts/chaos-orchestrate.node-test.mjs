import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { test } from "node:test";

function run(...args) {
  return spawnSync("node", ["scripts/chaos-orchestrate.mjs", ...args], { cwd: process.cwd(), encoding: "utf8" });
}

test("chaos plan validates and prints the bounded Timescale pause experiment", () => {
  const result = run("plan");
  assert.equal(result.status, 0, result.stderr);
  const plan = JSON.parse(result.stdout);
  assert.equal(plan.id, "simops-timescale-writer-pause");
  assert.equal(plan.scope.dockerContext, "orbstack");
  assert.equal(plan.fault.kind, "container-pause");
  assert.match(plan.execution, /--execute/);
});

test("chaos runner refuses state changes without explicit execution consent", () => {
  const result = run("run");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /read-only until you append --execute/);
});
