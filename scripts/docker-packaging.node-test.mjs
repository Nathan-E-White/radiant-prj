import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";
import { evaluatePackagingEvidence } from "./docker-packaging-lib.mjs";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("Docker packaging stays narrow, reproducible, and budgeted", () => {
  const dockerignore = read(".dockerignore");
  for (const ignored of [
    ".git",
    "node_modules",
    "dist",
    "storybook-static",
    "coverage",
    "test",
    "**/test",
    "tests",
    "**/tests",
    "test-results",
    "playwright-report",
    ".stryker-tmp",
    ".local",
    "generated",
    "target",
    ".terraform",
    ".tofu-artifacts",
    "*.tsbuildinfo"
  ]) {
    assert.match(dockerignore, new RegExp(`^${escapeRegExp(ignored)}/?$`, "m"), `missing ignore rule: ${ignored}`);
  }
  for (const included of [
    "!workers/scada-standins/tests",
    "!workers/scada-standins/tests/**",
    "!workers/simops-generator/tests",
    "!workers/simops-generator/tests/**"
  ]) {
    assert.match(dockerignore, new RegExp(`^${escapeRegExp(included)}$`, "m"), `missing required test-tree exception: ${included}`);
  }

  const frontend = read("Dockerfile");
  assert.match(frontend, /^COPY package\.json bun\.lock \.\/$/m);
  assert.match(frontend, /^RUN bun install --frozen-lockfile$/m);
  assert.doesNotMatch(frontend, /bun\.lockb/);

  const worker = read("worker.Dockerfile");
  assert.doesNotMatch(worker, /bun install|package\.json|bun\.lock|node_modules/);
  assert.match(worker, /^COPY scripts\/mock-worker\.mjs scripts\/mock-worker\.mjs$/m);
  assert.match(worker, /^COPY src\/data\/readiness-fixtures\.json src\/data\/readiness-fixtures\.json$/m);

  const goDockerfile = read("deploy/slurm-gateway.Dockerfile");
  assert.doesNotMatch(goDockerfile, /AS full-runtime/);
  for (const target of Object.values(goRoleTargets)) {
    assert.match(goDockerfile, new RegExp(` AS ${escapeRegExp(target)}$`, "m"), `missing Go runtime target: ${target}`);
  }
  assert.equal((goDockerfile.match(/apk add --no-cache docker-cli/g) || []).length, 1);

  const compose = read("deploy/slurm-gateway.compose.yml");
  for (const [service, target] of Object.entries(goRoleTargets)) {
    const block = serviceBlock(compose, service);
    assert.match(block, new RegExp(`^      target: ${escapeRegExp(target)}$`, "m"), `${service} must select ${target}`);
  }

  const budgets = JSON.parse(read("config/docker-packaging-budgets.json"));
  const baseline = JSON.parse(read("config/docker-packaging-baseline.json"));
  assert.ok(baseline.buildContextBytes > budgets.buildContext.maxBytes);
  assert.ok(baseline.images["mock-worker"].sizeBytes > 0);
  assert.ok(baseline.images["shared-go-full-runtime"].sizeBytes > 0);
  assert.equal(budgets.buildContext.maxBytes, 20 * 1024 * 1024);
  assert.ok(budgets.builderCache.maxGrowthBytes > 0);
  assert.ok(budgets.builderCache.maxReclaimableBytes > 0);
  assert.ok(budgets.browserAssets.maxTotalRawBytes > 0);
  assert.ok(budgets.browserAssets.maxTotalGzipBytes > 0);
  for (const role of ["console", "mock-worker", ...Object.keys(goRoleTargets)]) {
    assert.ok(budgets.images[role]?.maxBytes > 0, `missing image budget: ${role}`);
  }
  const amd64Evidence = JSON.parse(read("docs/verification/docker-packaging-evidence-amd64.json"));
  assert.equal(amd64Evidence.source.workflowRunId, 29680581898);
  assert.equal(amd64Evidence.contentAssertions.mockWorkerDependencyTreeAbsent, true);
  assert.deepEqual(evaluatePackagingEvidence(amd64Evidence, budgets), []);
  for (const [role, image] of Object.entries(amd64Evidence.images)) {
    assert.equal(image.architecture, "amd64", `${role} must retain amd64 provenance`);
    const limit = budgets.images[role].maxBytesByArchitecture.amd64;
    const headroom = limit / image.sizeBytes - 1;
    assert.ok(headroom >= 0.2 && headroom <= 0.4, `${role} amd64 headroom must stay between 20% and 40%`);
  }

  const packageJson = JSON.parse(read("package.json"));
  const repositoryVerification = JSON.parse(read("config/repository-verification.json"));
  assert.equal(packageJson.scripts["docker:packaging:contract:check"], "node --test scripts/docker-packaging.node-test.mjs");
  assert.equal(packageJson.scripts["docker:packaging:verify"], "node scripts/verify-docker-packaging.mjs");
  assert.match(packageJson.scripts.ci, /repository:verify/);
  assert.deepEqual(
    repositoryVerification.claims.find(({ id }) => id === "docker-packaging.structured-budgets")?.evidence.command,
    ["node", "--test", "scripts/docker-packaging.node-test.mjs"],
  );
  assert.match(packageJson.scripts.ci, /backend:dataplane:test/);

  const workflow = read(".github/workflows/ci.yml");
  assert.match(workflow, /run: bun run docker:packaging:verify/);
  assert.match(workflow, /run: bun run backend:dataplane:test/);
  assert.match(read("scripts/simops-local-smoke.sh"), /run --rm --no-deps simops-webtransport-probe --endpoint/);
});

const goRoleTargets = {
  "slurm-gateway": "gateway-runtime",
  "simops-moq-gateway": "simops-stream-gateway-runtime",
  "simops-webtransport-probe": "simops-webtransport-probe-runtime",
  "simops-timescale-writer": "simops-timescale-writer-runtime",
  "simops-iceberg-writer": "simops-iceberg-writer-runtime",
  "workbench-projection-writer": "workbench-projection-writer-runtime",
  "twin-projector": "twin-projector-runtime",
  "workbench-iceberg-writer": "workbench-iceberg-writer-runtime"
};

function serviceBlock(compose, service) {
  const start = compose.indexOf(`  ${service}:\n`);
  assert.notEqual(start, -1, `missing Compose service: ${service}`);
  const remainder = compose.slice(start + service.length + 4);
  const next = remainder.search(/^  [a-zA-Z0-9_-]+:\n/m);
  return next === -1 ? remainder : remainder.slice(0, next);
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
