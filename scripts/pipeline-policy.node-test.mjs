import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse as parseYaml } from "yaml";
import { verifyPipelinePolicy } from "./pipeline-policy/policy.mjs";

const ledger = { capabilities: [{ id: "ci-verification-invocation" }, { id: "docker-packaging-delivery" }] };
const policy = { schemaVersion: "radiant.pipeline-policy.v1", policies: [
  { id: "ci", capabilityId: "ci-verification-invocation", workflow: "ci.yml", permissions: { contents: "read", packages: "read" }, requiredRuns: ["bun run repository:verify"], untrustedPullRequest: { allowActionWhen: { "docker/login-action@v3": "safe-condition" } } },
  { id: "publish", capabilityId: "docker-packaging-delivery", workflow: "publish.yml", permissions: { contents: "read", packages: "write" }, requiredRuns: ["bun run docker:packaging:verify"], trustedPublication: { event: "push", branches: ["main"] } }
] };
const workflows = {
  "ci.yml": { on: { pull_request: null }, permissions: { contents: "read", packages: "read" }, jobs: { verify: { steps: [{ uses: "docker/login-action@v3", if: "safe-condition" }, { run: "bun run repository:verify" }] } } },
  "publish.yml": { on: { push: { branches: ["main"] } }, permissions: { contents: "read", packages: "write" }, jobs: { publish: { steps: [{ run: "bun run docker:packaging:verify" }] } } }
};

test("declares least privilege, read-only untrusted verification, and trusted publication", () => assert.equal(verifyPipelinePolicy({ policy, ledger, workflows }).exitCode, 0));

test("reports changed permissions, trust rules, invocations, and ledger mismatches actionably", () => {
  const invalid = structuredClone(workflows);
  invalid["ci.yml"].permissions.packages = "write";
  invalid["ci.yml"].jobs.verify.steps[0].if = "true";
  invalid["publish.yml"].on.push.branches = ["feature"];
  invalid["publish.yml"].jobs.publish.steps = [];
  const workflowReport = verifyPipelinePolicy({ policy, ledger, workflows: invalid });
  assert.equal(workflowReport.exitCode, 1);
  const fields = workflowReport.findings.map(({ field }) => field);
  for (const field of ["permissions.packages", "untrustedPullRequest.docker/login-action@v3", "trustedPublication", "requiredRuns"]) assert.ok(fields.includes(field), `expected actionable ${field} failure`);
  const brokenPolicy = structuredClone(policy);
  brokenPolicy.policies[0].capabilityId = "missing";
  const ledgerReport = verifyPipelinePolicy({ policy: brokenPolicy, ledger, workflows });
  assert.equal(ledgerReport.exitCode, 1);
  assert.equal(ledgerReport.findings[0].field, "capabilityId");
});

test("the production declarations parse and pass against their workflows", () => {
  const productionPolicy = JSON.parse(readFileSync("config/pipeline-policy.json", "utf8"));
  const productionLedger = JSON.parse(readFileSync("config/capability-ledger.json", "utf8"));
  const productionWorkflows = Object.fromEntries(productionPolicy.policies.map(({ workflow }) => [workflow, parseYaml(readFileSync(workflow, "utf8"))]));
  assert.equal(verifyPipelinePolicy({ policy: productionPolicy, ledger: productionLedger, workflows: productionWorkflows }).exitCode, 0);
});

test("CI runs the same stable local policy command", () => assert.match(readFileSync(".github/workflows/ci.yml", "utf8"), /run: bun run pipeline:policy:check/));
