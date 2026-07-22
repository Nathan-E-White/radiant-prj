#!/usr/bin/env node
import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { dirname, extname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { buildInputIdentity, evaluatePackagingEvidence, parseBuildContextBytes, parseByteSize, summarizeBuildxBake } from "./docker-packaging-lib.mjs";

const repoRoot = resolve(new URL("..", import.meta.url).pathname);
const dockerContext = process.env.DOCKER_CONTEXT || (process.env.CI ? "default" : "orbstack");
const outputPath = resolve(repoRoot, outputArgument() || ".local/docker-packaging-evidence/latest.json");
const budgets = JSON.parse(readFileSync(join(repoRoot, "config/docker-packaging-budgets.json"), "utf8"));
const baseline = JSON.parse(readFileSync(join(repoRoot, "config/docker-packaging-baseline.json"), "utf8"));
const inputManifest = JSON.parse(readFileSync(join(repoRoot, "config/docker-packaging-inputs.json"), "utf8"));
const tagPrefix = "radiant-packaging";
const images = {};
const imageAliases = {
  "reactor-telemetry-worker": ["radiant-scada-standins:ci", "radiant-scada-standins:latest"],
  "simops-generator": ["radiant-simops-generator:latest"]
};
const platform = process.env.DOCKER_PACKAGING_PLATFORM || process.env.DOCKER_DEFAULT_PLATFORM || (process.env.CI ? "linux/amd64" : nativePlatform());
const registryReuseEnabled = truthy(process.env.DOCKER_PACKAGING_REGISTRY_REUSE);
const registryPublishEnabled = truthy(process.env.DOCKER_PACKAGING_REGISTRY_PUBLISH);
let publishFailureCount = 0;

const roles = [
  { role: "console", dockerfile: "Dockerfile", family: "CONSOLE" },
  { role: "mock-worker", dockerfile: "worker.Dockerfile", family: "MOCK_WORKER" },
  { role: "reactor-telemetry-worker", dockerfile: "deploy/scada-standins.Dockerfile", family: "REACTOR_TELEMETRY_WORKER" },
  { role: "simops-generator", dockerfile: "deploy/simops-generator.Dockerfile", family: "SIMOPS_GENERATOR" },
  { role: "slurm-gateway", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "gateway-runtime", family: "GO_RUNTIME" },
  { role: "simops-moq-gateway", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-stream-gateway-runtime", family: "GO_RUNTIME" },
  { role: "simops-webtransport-probe", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-webtransport-probe-runtime", family: "GO_RUNTIME" },
  { role: "simops-timescale-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-timescale-writer-runtime", family: "GO_RUNTIME" },
  { role: "simops-iceberg-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-iceberg-writer-runtime", family: "GO_RUNTIME" },
  { role: "workbench-projection-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "workbench-projection-writer-runtime", family: "GO_RUNTIME" },
  { role: "twin-projector", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "twin-projector-runtime", family: "GO_RUNTIME" },
  { role: "workbench-iceberg-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "workbench-iceberg-writer-runtime", family: "GO_RUNTIME" }
];
const cacheFamilies = {
  CONSOLE: {
    fromEnv: "DOCKER_BAKE_CACHE_FROM_CONSOLE",
    toEnv: "DOCKER_BAKE_CACHE_TO_CONSOLE"
  },
  MOCK_WORKER: {
    fromEnv: "DOCKER_BAKE_CACHE_FROM_MOCK_WORKER",
    toEnv: "DOCKER_BAKE_CACHE_TO_MOCK_WORKER"
  },
  REACTOR_TELEMETRY_WORKER: {
    fromEnv: "DOCKER_BAKE_CACHE_FROM_REACTOR_TELEMETRY_WORKER",
    toEnv: "DOCKER_BAKE_CACHE_TO_REACTOR_TELEMETRY_WORKER"
  },
  SIMOPS_GENERATOR: {
    fromEnv: "DOCKER_BAKE_CACHE_FROM_SIMOPS_GENERATOR",
    toEnv: "DOCKER_BAKE_CACHE_TO_SIMOPS_GENERATOR"
  },
  GO_RUNTIME: {
    fromEnv: "DOCKER_BAKE_CACHE_FROM_GO_RUNTIME",
    toEnv: "DOCKER_BAKE_CACHE_TO_GO_RUNTIME"
  }
};

console.log(`Docker packaging verification: context=${dockerContext}`);
run("bun", ["run", "build"]);
mkdirSync(dirname(outputPath), { recursive: true });
const cacheBefore = dockerStorage().buildCache;
const identities = Object.fromEntries(roles.map((role) => [
  role.role,
  buildInputIdentity(inputManifest, role.role, { root: repoRoot, platform })
]));
const registry = resolveRegistryImages();
const buildRoles = roles.filter((role) => registry[role.role].status !== "hit");

const buildStartedAt = Date.now();
const buildResult = buildRoles.length === 0
  ? { stdout: "", stderr: "", status: 0 }
  : runDocker([
    "buildx", "bake",
    "-f", "docker-bake.hcl",
    "--load",
    "--progress=plain",
    ...cacheSetArgs(buildRoles),
    ...platformSetArgs(buildRoles),
    ...baseImageSetArgs(buildRoles),
    ...buildRoles.map((role) => role.role)
  ], { capture: true });
const buildElapsedMs = Date.now() - buildStartedAt;
const buildLog = `${buildResult.stdout}\n${buildResult.stderr}`;
const buildLogPath = outputPath.replace(/\.json$/, "-build.log");
writeFileSync(buildLogPath, buildLog);
const buildContextBytes = buildRoles.length === 0 ? 0 : parseBuildContextBytes(buildLog);
const buildSummary = summarizeBuildxBake(buildLog);

for (const role of roles) {
  const tag = `${tagPrefix}-${role.role}:verify`;
  if (registry[role.role].status !== "hit") publishRegistryImage(role, tag);
  const executionRef = registry[role.role].resolvedDigest || tag;
  for (const alias of imageAliases[role.role] ?? []) runDocker(["image", "tag", tag, alias]);
  const sizeBytes = Number(runDocker(["image", "inspect", executionRef, "--format", "{{.Size}}"], { capture: true }).stdout.trim());
  const architecture = runDocker(["image", "inspect", executionRef, "--format", "{{.Architecture}}"], { capture: true }).stdout.trim();
  images[role.role] = {
    tag,
    executionRef,
    aliases: imageAliases[role.role] ?? [],
    dockerfile: role.dockerfile,
    target: role.target || null,
    family: role.family,
    architecture,
    sizeBytes,
    registry: registry[role.role]
  };
}

verifyImageContents(images);
const cacheAfter = dockerStorage().buildCache;
const browserAssets = measureBrowserAssets(join(repoRoot, "dist"));
const evidence = {
  schemaVersion: 1,
  measuredAt: new Date().toISOString(),
  dockerContext,
  buildContextBytes,
  build: {
    tool: "docker buildx bake",
    bakeFile: "docker-bake.hcl",
    group: "packaging",
    targets: buildRoles.map((role) => role.role),
    platform,
    elapsedMs: buildElapsedMs,
    logPath: buildLogPath.slice(repoRoot.length + 1),
    cachePolicy: cachePolicyEvidence(),
    registryReuseEnabled,
    registryPublishEnabled,
    registryHitCount: Object.values(registry).filter((entry) => entry.status === "hit").length,
    registryMissCount: Object.values(registry).filter((entry) => entry.status === "miss").length,
    registryFallbackCount: Object.values(registry).filter((entry) => entry.status === "fallback").length,
    ...buildSummary
  },
  images,
  registryRetention: registryRetentionEvidence(images),
  builderCache: {
    beforeBytes: cacheBefore.sizeBytes,
    aggregateBytes: cacheAfter.sizeBytes,
    growthBytes: Math.max(0, cacheAfter.sizeBytes - cacheBefore.sizeBytes),
    reclaimableBytes: cacheAfter.reclaimableBytes
  },
  browserAssets
};
const violations = evaluatePackagingEvidence(evidence, budgets);
if (registryPublishEnabled && publishFailureCount > 0) {
  violations.push(`${publishFailureCount} Docker packaging image publish operation failed`);
}
const report = { ...evidence, baseline, budgets, verdict: violations.length === 0 ? "passed" : "failed", violations };
mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, `${JSON.stringify(report, null, 2)}\n`);

console.log(`Evidence: ${outputPath}`);
console.log(`Build context: ${buildContextBytes} bytes`);
for (const [role, image] of Object.entries(images)) console.log(`Image ${role}: ${image.sizeBytes} bytes`);
console.log(`Builder cache: growth=${evidence.builderCache.growthBytes} aggregate=${evidence.builderCache.aggregateBytes} reclaimable=${evidence.builderCache.reclaimableBytes}`);
console.log(`Browser assets: raw=${browserAssets.totalRawBytes} gzip=${browserAssets.totalGzipBytes}`);
if (violations.length) {
  console.error("Docker packaging budgets failed:");
  for (const violation of violations) console.error(`- ${violation}`);
  process.exit(1);
}
console.log("Docker packaging budgets passed.");

function runDocker(args, options = {}) {
  return run("docker", ["--context", dockerContext, ...args], options);
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
    env: { ...process.env, DOCKER_BUILDKIT: "1" },
    maxBuffer: 64 * 1024 * 1024
  });
  if (options.capture && result.stdout) process.stdout.write(result.stdout);
  if (options.capture && result.stderr) process.stderr.write(result.stderr);
  if (!options.allowFailure && (result.error || result.status !== 0)) {
    throw new Error(`${command} ${args.join(" ")} failed: ${result.error?.message || result.stderr || result.stdout}`);
  }
  return result;
}

function dockerStorage() {
  const result = runDocker(["system", "df", "--format", "{{json .}}"], { capture: true });
  const rows = result.stdout.trim().split(/\r?\n/).filter(Boolean).map((line) => JSON.parse(line));
  const buildCache = rows.find((row) => row.Type === "Build Cache");
  if (!buildCache) throw new Error("docker system df did not report Build Cache");
  return {
    buildCache: {
      sizeBytes: parseByteSize(buildCache.Size),
      reclaimableBytes: parseByteSize(String(buildCache.Reclaimable).split(/\s+/)[0])
    }
  };
}

function verifyImageContents(builtImages) {
  runDocker(["run", "--rm", builtImages["mock-worker"].executionRef]);
  runDocker([
    "run", "--rm", "--entrypoint", "sh", builtImages["mock-worker"].executionRef, "-c",
    "test ! -e /worker/node_modules && test ! -e /worker/package.json && test ! -e /worker/bun.lock"
  ]);
  runDocker(["run", "--rm", builtImages["reactor-telemetry-worker"].executionRef, "--source-id", "SRC-PACKAGING-CHECK", "--max-frames", "1"]);
  runDocker(["run", "--rm", builtImages["simops-generator"].executionRef, "--help"]);

  const binaries = {
    "slurm-gateway": "slurm-gateway",
    "simops-moq-gateway": "simops-stream-gateway",
    "simops-webtransport-probe": "simops-webtransport-probe",
    "simops-timescale-writer": "simops-timescale-writer",
    "simops-iceberg-writer": "simops-iceberg-writer",
    "workbench-projection-writer": "workbench-projection-writer",
    "twin-projector": "twin-projector",
    "workbench-iceberg-writer": "workbench-iceberg-writer"
  };
  for (const [role, binary] of Object.entries(binaries)) {
    const dockerExpectation = role === "slurm-gateway" ? "command -v docker >/dev/null" : "! command -v docker >/dev/null";
    runDocker([
      "run", "--rm", "--entrypoint", "/bin/sh", builtImages[role].executionRef, "-c",
      `test -x /app/${binary} && test "$(find /app -maxdepth 1 -type f | wc -l | tr -d ' ')" = 1 && ${dockerExpectation}`
    ]);
  }
}

function measureBrowserAssets(root) {
  if (!existsSync(root)) throw new Error("Production dist directory is missing after build");
  const files = walk(root).map((path) => {
    const bytes = readFileSync(path);
    return { path: path.slice(root.length + 1), rawBytes: bytes.length, gzipBytes: gzipSync(bytes).length };
  });
  const javaScript = files.filter((file) => extname(file.path) === ".js");
  return {
    totalRawBytes: files.reduce((sum, file) => sum + file.rawBytes, 0),
    totalGzipBytes: files.reduce((sum, file) => sum + file.gzipBytes, 0),
    maxJavaScriptChunkBytes: Math.max(0, ...javaScript.map((file) => file.rawBytes)),
    maxSingleAssetBytes: Math.max(0, ...files.map((file) => file.rawBytes)),
    files: files.sort((a, b) => b.rawBytes - a.rawBytes)
  };
}

function walk(root) {
  return readdirSync(root).flatMap((name) => {
    const path = join(root, name);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });
}

function outputArgument() {
  const index = process.argv.indexOf("--output");
  if (index === -1) return null;
  if (!process.argv[index + 1]) throw new Error("--output requires a path");
  return process.argv[index + 1];
}

function resolveRegistryImages() {
  return Object.fromEntries(roles.map((role) => {
    const identity = identities[role.role];
    const base = {
      inputKey: identity.key,
      registryRef: identity.registryRef,
      platform,
      status: registryReuseEnabled ? "miss" : "disabled",
      resolvedDigest: null,
      fallbackReason: registryReuseEnabled ? "registry image was not resolved before build" : "registry reuse disabled",
      publish: { attempted: false, status: "skipped", reason: registryPublishEnabled ? "image was not built yet" : "publishing disabled" }
    };
    if (!registryReuseEnabled) return [role.role, base];

    const inspect = runDocker(["manifest", "inspect", identity.registryRef], { allowFailure: true });
    if (inspect.status !== 0) {
      const diagnostic = conciseDockerDiagnostic(inspect);
      return [role.role, {
        ...base,
        status: diagnostic.match(/no such manifest|manifest unknown|not found/i) ? "miss" : "fallback",
        fallbackReason: diagnostic || "registry manifest inspect failed"
      }];
    }
    const pullStartedAt = Date.now();
    const pull = runDocker(["pull", "--platform", platform, identity.registryRef], { capture: true, allowFailure: true });
    if (pull.status !== 0) {
      return [role.role, {
        ...base,
        status: "fallback",
        fallbackReason: conciseDockerDiagnostic(pull) || "registry pull failed"
      }];
    }
    const tag = `${tagPrefix}-${role.role}:verify`;
    runDocker(["image", "tag", identity.registryRef, tag]);
    return [role.role, {
      ...base,
      status: "hit",
      resolvedDigest: imageDigest(identity.registryRef),
      fallbackReason: null,
      pullElapsedMs: Date.now() - pullStartedAt
    }];
  }));
}

function publishRegistryImage(role, tag) {
  const entry = registry[role.role];
  if (!registryPublishEnabled) return;
  entry.publish = { attempted: true, status: "pending" };
  runDocker(["image", "tag", tag, entry.registryRef]);
  const push = runDocker(["push", entry.registryRef], { capture: true, allowFailure: true });
  if (push.status !== 0) {
    entry.publish = { attempted: true, status: "failed", reason: conciseDockerDiagnostic(push) || "registry push failed" };
    publishFailureCount += 1;
    return;
  }
  entry.publish = { attempted: true, status: "published" };
  entry.resolvedDigest = parsePushedDigest(push.stdout, entry.registryRef) || imageDigest(entry.registryRef);
}

function imageDigest(ref) {
  const result = runDocker(["image", "inspect", ref, "--format", "{{json .RepoDigests}}"], { allowFailure: true });
  if (result.status !== 0) return null;
  const digests = JSON.parse(result.stdout.trim() || "[]");
  return digests[0] || null;
}

function parsePushedDigest(output, ref) {
  const match = String(output).match(/digest:\s+(sha256:[a-f0-9]{64})/i);
  if (!match) return null;
  return `${ref.replace(/:[^/:]+$/, "")}@${match[1]}`;
}

function conciseDockerDiagnostic(result) {
  return `${result.stderr || ""}\n${result.stdout || ""}`.trim().split(/\r?\n/).slice(-4).join(" | ");
}

function registryRetentionEvidence(resolvedImages) {
  const policy = inputManifest.registry?.retention ?? {};
  const currentRunImageBytes = Object.values(resolvedImages).reduce((sum, image) => sum + image.sizeBytes, 0);
  const retainedStorageBytes = numericEnv("DOCKER_PACKAGING_REGISTRY_RETAINED_BYTES");
  const allowanceBytes = Number(policy.maxTotalBytes || 0);
  return {
    policy,
    allowanceBytes,
    retainedStorageBytes,
    currentRunImageBytes,
    reportedBytes: retainedStorageBytes ?? currentRunImageBytes,
    withinAllowance: allowanceBytes > 0 ? (retainedStorageBytes ?? currentRunImageBytes) <= allowanceBytes : null,
    measurement: retainedStorageBytes === null
      ? "current run image total used until GHCR retained storage inventory is supplied"
      : "retained GHCR package storage supplied by workflow"
  };
}

function numericEnv(name) {
  const value = process.env[name];
  if (value === undefined || value === "") return null;
  const number = Number(value);
  if (!Number.isFinite(number) || number < 0) throw new Error(`${name} must be a non-negative number`);
  return number;
}

function cacheSetArgs(targetRoles = roles) {
  return Object.entries(cacheFamilies).flatMap(([family, env]) => {
    const targets = targetRoles.filter((role) => role.family === family).map((role) => role.role);
    const from = cachePolicyValue(env.fromEnv);
    const to = cachePolicyValue(env.toEnv);
    return [
      ...targets.flatMap((target) => from ? ["--set", `${target}.cache-from=${from}`] : []),
      ...targets.flatMap((target) => to ? ["--set", `${target}.cache-to=${to}`] : [])
    ];
  });
}

function platformSetArgs(targetRoles = roles) {
  return targetRoles.flatMap((role) => ["--set", `${role.role}.platform=${platform}`]);
}

function baseImageSetArgs(targetRoles = roles) {
  return targetRoles.flatMap((role) => {
    const image = inputManifest.images.find((candidate) => candidate.role === role.role);
    return Object.entries(image?.baseImageBuildArgs ?? {}).flatMap(([baseImageName, buildArg]) => {
      const baseImage = inputManifest.baseImages?.[baseImageName];
      const digest = baseImage?.digestByPlatform?.[platform];
      if (!baseImage || !digest) throw new Error(`Missing pinned ${platform} base image for ${role.role}:${baseImageName}`);
      return ["--set", `${role.role}.args.${buildArg}=${baseImage.ref}@${digest}`];
    });
  });
}

function cachePolicyEvidence() {
  return Object.fromEntries(
    Object.entries(cacheFamilies)
      .flatMap(([family, env]) => [
        [`${family}.cacheFrom`, cachePolicyValue(env.fromEnv)],
        [`${family}.cacheTo`, cachePolicyValue(env.toEnv)]
      ])
      .filter(([, value]) => value)
  );
}

function cachePolicyValue(name) {
  return (process.env[name] || "").trim();
}

function truthy(value) {
  return ["1", "true", "yes"].includes(String(value || "").toLowerCase());
}

function nativePlatform() {
  const architecture = process.arch === "arm64" ? "arm64" : "amd64";
  return `linux/${architecture}`;
}
