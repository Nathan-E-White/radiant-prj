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
  assert.match(frontend, /^ARG CONSOLE_BUN_IMAGE=oven\/bun:1\.3\.14$/m);
  assert.match(frontend, /^ARG CONSOLE_NGINX_IMAGE=nginx:1\.27-alpine$/m);
  assert.match(frontend, /^FROM \$\{CONSOLE_BUN_IMAGE\} AS deps$/m);
  assert.match(frontend, /^FROM \$\{CONSOLE_NGINX_IMAGE\}$/m);
  assert.match(frontend, /^COPY package\.json bun\.lock \.\/$/m);
  assert.match(frontend, /^RUN bun install --frozen-lockfile$/m);
  assert.doesNotMatch(frontend, /bun\.lockb/);

  const worker = read("worker.Dockerfile");
  assert.match(worker, /^ARG MOCK_WORKER_BUN_IMAGE=oven\/bun:1\.3\.14$/m);
  assert.match(worker, /^FROM \$\{MOCK_WORKER_BUN_IMAGE\}$/m);
  assert.doesNotMatch(worker, /bun install|package\.json|bun\.lock|node_modules/);
  assert.match(worker, /^COPY scripts\/mock-worker\.mjs scripts\/mock-worker\.mjs$/m);
  assert.match(worker, /^COPY src\/data\/readiness-fixtures\.json src\/data\/readiness-fixtures\.json$/m);

  const goDockerfile = read("deploy/slurm-gateway.Dockerfile");
  assert.doesNotMatch(goDockerfile, /AS full-runtime/);
  for (const target of Object.values(goRoleTargets)) {
    assert.match(goDockerfile, new RegExp(` AS ${escapeRegExp(target)}$`, "m"), `missing Go runtime target: ${target}`);
  }
  assert.equal((goDockerfile.match(/id=simops-go-modules,target=\/go\/pkg\/mod,sharing=locked/g) || []).length, 9);
  assert.equal((goDockerfile.match(/id=simops-go-build,target=\/root\/.cache\/go-build,sharing=locked/g) || []).length, 9);
  assert.equal((goDockerfile.match(/apk add --no-cache docker-cli/g) || []).length, 1);

  const bake = read("docker-bake.hcl");
  const packagingRoles = ["console", "mock-worker", "reactor-telemetry-worker", "simops-generator", ...Object.keys(goRoleTargets)];
  for (const role of packagingRoles) {
    assert.match(bake, new RegExp(`target "${escapeRegExp(role)}"`), `missing Bake target: ${role}`);
    assert.match(bake, new RegExp(`radiant-packaging-${escapeRegExp(role)}:verify`), `missing verification tag for ${role}`);
  }
  assert.match(bake, /group "packaging"/);
  assert.match(bake, /dockerfile = "deploy\/scada-standins\.Dockerfile"/);
  assert.match(bake, /dockerfile = "deploy\/simops-generator\.Dockerfile"/);

  const compose = read("deploy/slurm-gateway.compose.yml");
  for (const [service, target] of Object.entries(goRoleTargets)) {
    const block = serviceBlock(compose, service);
    assert.match(block, new RegExp(`^      target: ${escapeRegExp(target)}$`, "m"), `${service} must select ${target}`);
  }

  const budgets = JSON.parse(read("config/docker-packaging-budgets.json"));
  const inputManifest = JSON.parse(read("config/docker-packaging-inputs.json"));
  assert.equal(inputManifest.schemaVersion, "radiant.docker-packaging-inputs.v1");
  assert.deepEqual(inputManifest.defaults.platforms, ["linux/amd64", "linux/arm64"]);
  assert.equal(inputManifest.registry.host, "ghcr.io");
  assert.equal(inputManifest.registry.owner, "nathan-e-white");
  assert.ok(inputManifest.registry.retention.maxVersionsPerImage > 0);
  assert.ok(inputManifest.registry.retention.maxTotalBytes > 0);
  for (const [name, baseImage] of Object.entries(inputManifest.baseImages)) {
    assert.match(baseImage.ref, /^[^@]+:[^@]+$/, `${name} must retain the human-readable base image tag`);
    for (const platform of inputManifest.defaults.platforms) {
      assert.match(baseImage.digestByPlatform[platform], /^sha256:[a-f0-9]{64}$/, `${name} missing pinned digest for ${platform}`);
    }
  }
  for (const role of packagingRoles) {
    const image = inputManifest.images.find((candidate) => candidate.role === role);
    assert.ok(image, `missing build-input manifest image: ${role}`);
    assert.equal(image.bakeTarget, role);
    assert.ok(image.inputs.length > 0, `${role} must declare build inputs`);
    assert.ok(image.inputs.some((input) => input.path === ".dockerignore"), `${role} must key Docker context filtering`);
    assert.ok(image.baseImages.length > 0, `${role} must declare base-image inputs`);
    assert.deepEqual(Object.keys(image.baseImageBuildArgs).sort(), [...image.baseImages].sort(), `${role} must map every base image to a build arg`);
  }
  const baseline = JSON.parse(read("config/docker-packaging-baseline.json"));
  assert.ok(baseline.buildContextBytes > budgets.buildContext.maxBytes);
  assert.ok(baseline.images["mock-worker"].sizeBytes > 0);
  assert.ok(baseline.images["shared-go-full-runtime"].sizeBytes > 0);
  assert.equal(budgets.buildContext.maxBytes, 20 * 1024 * 1024);
  assert.ok(budgets.builderCache.maxGrowthBytes > 0);
  assert.ok(budgets.builderCache.maxReclaimableBytes > 0);
  assert.ok(budgets.browserAssets.maxTotalRawBytes > 0);
  assert.ok(budgets.browserAssets.maxTotalGzipBytes > 0);
  for (const role of ["console", "mock-worker", "reactor-telemetry-worker", "simops-generator", ...Object.keys(goRoleTargets)]) {
    assert.ok(budgets.images[role]?.maxBytes > 0, `missing image budget: ${role}`);
  }
  const amd64Evidence = JSON.parse(read("docs/verification/docker-packaging-evidence-amd64.json"));
  assert.equal(amd64Evidence.source.workflowRunId, 29680581898);
  assert.equal(amd64Evidence.contentAssertions.mockWorkerDependencyTreeAbsent, true);
  const amd64HistoricalBudgets = {
    ...budgets,
    images: Object.fromEntries(Object.entries(budgets.images).filter(([role]) => role in amd64Evidence.images))
  };
  assert.deepEqual(evaluatePackagingEvidence(amd64Evidence, amd64HistoricalBudgets), []);
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

  const verifier = read("scripts/verify-docker-packaging.mjs");
  assert.match(verifier, /"buildx", "bake"/);
  assert.match(verifier, /config\/docker-packaging-inputs\.json/);
  assert.match(verifier, /buildInputIdentity/);
  assert.match(verifier, /"manifest", "inspect"/);
  assert.match(verifier, /"pull"/);
  assert.match(verifier, /"--platform", platform/);
  assert.match(verifier, /"push"/);
  assert.match(verifier, /DOCKER_PACKAGING_REGISTRY_REUSE/);
  assert.match(verifier, /DOCKER_PACKAGING_REGISTRY_PUBLISH/);
  assert.match(verifier, /baseImageSetArgs/);
  assert.match(verifier, /platformSetArgs/);
  assert.match(verifier, /\.platform=\$\{platform\}/);
  assert.match(verifier, /\.args\.\$\{buildArg\}/);
  assert.match(verifier, /inputKey/);
  assert.match(verifier, /resolvedDigest/);
  assert.match(verifier, /executionRef/);
  assert.match(verifier, /registryRetentionEvidence/);
  assert.match(verifier, /DOCKER_PACKAGING_REGISTRY_RETAINED_BYTES/);
  assert.match(verifier, /currentRunImageBytes/);
  assert.doesNotMatch(verifier, /for \(const role of roles\)[\s\S]*"build", "--progress=plain"/);
  assert.match(verifier, /DOCKER_BAKE_CACHE_FROM_GO_RUNTIME/);
  assert.match(verifier, /DOCKER_BAKE_CACHE_TO_GO_RUNTIME/);
  assert.match(verifier, /radiant-scada-standins:ci/);
  assert.match(verifier, /radiant-simops-generator:latest/);
  assert.match(read("scripts/docker-packaging-lib.mjs"), /cacheExportErrorCount/);

  const workflow = read(".github/workflows/ci.yml");
  assert.match(workflow, /packages: read/);
  assert.match(workflow, /docker\/login-action@v3/);
  assert.match(workflow, /github\.event_name != 'pull_request' \|\| github\.event\.pull_request\.head\.repo\.full_name == github\.repository/);
  assert.match(workflow, /docker\/setup-buildx-action@v3/);
  assert.doesNotMatch(workflow, /Build Reactor Telemetry worker image/);
  assert.doesNotMatch(workflow, /run: docker build -f deploy\/scada-standins\.Dockerfile/);
  assert.match(workflow, /run: bun run docker:packaging:verify/);
  assert.match(workflow, /DOCKER_BAKE_CACHE_FROM_GO_RUNTIME: type=gha,scope=radiant-go-runtime-main/);
  assert.match(workflow, /DOCKER_BAKE_CACHE_TO_GO_RUNTIME: \$\{\{ github\.ref == 'refs\/heads\/main' && 'type=gha,scope=radiant-go-runtime-main,mode=max,ignore-error=true' \|\| '' \}\}/);
  assert.match(workflow, /DOCKER_PACKAGING_REGISTRY_REUSE: "true"/);
  assert.match(workflow, /DOCKER_PACKAGING_REGISTRY_PUBLISH: "false"/);
  assert.match(workflow, /run: bun run backend:dataplane:test/);
  const publishWorkflow = read(".github/workflows/docker-packaging-publish.yml");
  assert.match(publishWorkflow, /branches:\n      - main/);
  assert.match(publishWorkflow, /packages: write/);
  assert.match(publishWorkflow, /docker\/login-action@v3/);
  assert.match(publishWorkflow, /run: bun run docker:packaging:verify/);
  assert.match(publishWorkflow, /DOCKER_PACKAGING_REGISTRY_REUSE: "true"/);
  assert.match(publishWorkflow, /DOCKER_PACKAGING_REGISTRY_PUBLISH: "true"/);
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
