#!/usr/bin/env node
import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { dirname, extname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { evaluatePackagingEvidence, parseBuildContextBytes, parseByteSize } from "./docker-packaging-lib.mjs";

const repoRoot = resolve(new URL("..", import.meta.url).pathname);
const dockerContext = process.env.DOCKER_CONTEXT || (process.env.CI ? "default" : "orbstack");
const outputPath = resolve(repoRoot, outputArgument() || ".local/docker-packaging-evidence/latest.json");
const budgets = JSON.parse(readFileSync(join(repoRoot, "config/docker-packaging-budgets.json"), "utf8"));
const baseline = JSON.parse(readFileSync(join(repoRoot, "config/docker-packaging-baseline.json"), "utf8"));
const tagPrefix = "radiant-packaging";
const images = {};

const roles = [
  { role: "console", dockerfile: "Dockerfile" },
  { role: "mock-worker", dockerfile: "worker.Dockerfile" },
  { role: "slurm-gateway", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "gateway-runtime" },
  { role: "simops-moq-gateway", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-stream-gateway-runtime" },
  { role: "simops-webtransport-probe", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-webtransport-probe-runtime" },
  { role: "simops-timescale-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-timescale-writer-runtime" },
  { role: "simops-iceberg-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "simops-iceberg-writer-runtime" },
  { role: "workbench-projection-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "workbench-projection-writer-runtime" },
  { role: "twin-projector", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "twin-projector-runtime" },
  { role: "workbench-iceberg-writer", dockerfile: "deploy/slurm-gateway.Dockerfile", target: "workbench-iceberg-writer-runtime" }
];

console.log(`Docker packaging verification: context=${dockerContext}`);
run("bun", ["run", "build"]);
const cacheBefore = dockerStorage().buildCache;
let buildContextBytes;

for (const role of roles) {
  const tag = `${tagPrefix}-${role.role}:verify`;
  const args = ["build", "--progress=plain", "-f", role.dockerfile, "-t", tag];
  if (role.target) args.push("--target", role.target);
  args.push(".");
  const result = runDocker(args, { capture: true });
  if (role.role === "console") buildContextBytes = parseBuildContextBytes(`${result.stdout}\n${result.stderr}`);
  const sizeBytes = Number(runDocker(["image", "inspect", tag, "--format", "{{.Size}}"], { capture: true }).stdout.trim());
  const architecture = runDocker(["image", "inspect", tag, "--format", "{{.Architecture}}"], { capture: true }).stdout.trim();
  images[role.role] = { tag, target: role.target || null, architecture, sizeBytes };
}

verifyImageContents(images);
const cacheAfter = dockerStorage().buildCache;
const browserAssets = measureBrowserAssets(join(repoRoot, "dist"));
const evidence = {
  schemaVersion: 1,
  measuredAt: new Date().toISOString(),
  dockerContext,
  buildContextBytes,
  images,
  builderCache: {
    beforeBytes: cacheBefore.sizeBytes,
    aggregateBytes: cacheAfter.sizeBytes,
    growthBytes: Math.max(0, cacheAfter.sizeBytes - cacheBefore.sizeBytes),
    reclaimableBytes: cacheAfter.reclaimableBytes
  },
  browserAssets
};
const violations = evaluatePackagingEvidence(evidence, budgets);
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
  if (result.error || result.status !== 0) {
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
  runDocker(["run", "--rm", builtImages["mock-worker"].tag]);
  runDocker([
    "run", "--rm", "--entrypoint", "sh", builtImages["mock-worker"].tag, "-c",
    "test ! -e /worker/node_modules && test ! -e /worker/package.json && test ! -e /worker/bun.lock"
  ]);

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
      "run", "--rm", "--entrypoint", "/bin/sh", builtImages[role].tag, "-c",
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
