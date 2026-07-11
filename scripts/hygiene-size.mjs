#!/usr/bin/env node
import { accessSync, constants, existsSync, lstatSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import { delimiter, join, relative, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const defaultRepoRoot = fileURLToPath(new URL("..", import.meta.url)).replace(/\/$/, "");
const repoRoot = resolve(process.env.RADIANT_REPO_ROOT || defaultRepoRoot);
const gitBin = process.env.GIT_BIN || "git";
const dockerBin = process.env.DOCKER_BIN || "docker";
const dockerContext = process.env.DOCKER_CONTEXT || "orbstack";
const duBin = process.env.DU_BIN || "du";
const goBin = process.env.GO_BIN || "go";

const generatedOutputDirs = ["storybook-static", "dist", "generated", ".local"];
const scanSkipDirs = new Set([
  ".git",
  ".worktrees",
  "node_modules",
  "target",
  "dist",
  "generated",
  "storybook-static",
  ".local"
]);

function run(bin, args, options = {}) {
  const result = spawnSync(bin, args, {
    cwd: options.cwd || repoRoot,
    encoding: "utf8",
    env: process.env,
    timeout: options.timeoutMs || 30000
  });

  if (result.error) {
    return {
      ok: false,
      status: null,
      stdout: result.stdout || "",
      stderr: result.stderr || "",
      reason: result.error.message
    };
  }

  if (result.status !== 0) {
    return {
      ok: false,
      status: result.status,
      stdout: result.stdout || "",
      stderr: result.stderr || "",
      reason: firstLine(result.stderr || result.stdout) || `${bin} exited ${result.status}`
    };
  }

  return {
    ok: true,
    status: result.status,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    reason: ""
  };
}

function firstLine(value) {
  return String(value || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find(Boolean) || "";
}

function pathExists(targetPath) {
  try {
    return existsSync(targetPath);
  } catch {
    return false;
  }
}

function isReadable(targetPath) {
  try {
    accessSync(targetPath, constants.R_OK);
    return true;
  } catch {
    return false;
  }
}

function measurePath(targetPath) {
  if (!pathExists(targetPath)) {
    return { state: "missing", path: targetPath, bytes: 0, reason: "not present" };
  }

  if (!isReadable(targetPath)) {
    return { state: "skipped", path: targetPath, bytes: 0, reason: "not readable" };
  }

  const result = run(duBin, ["-sk", targetPath], { timeoutMs: 120000 });
  if (result.ok) {
    const match = result.stdout.match(/^\s*(\d+)\s+/);
    if (match) {
      return {
        state: "present",
        path: targetPath,
        bytes: Number(match[1]) * 1024,
        reason: ""
      };
    }
  }

  const fallback = measurePathWithNode(targetPath);
  if (fallback.state === "present") {
    return fallback;
  }

  return {
    state: "skipped",
    path: targetPath,
    bytes: 0,
    reason: result.reason || fallback.reason || "size unavailable"
  };
}

function measurePathWithNode(targetPath) {
  try {
    const stats = lstatSync(targetPath);
    if (stats.isSymbolicLink()) {
      return { state: "present", path: targetPath, bytes: stats.size, reason: "" };
    }
    if (stats.isFile()) {
      return { state: "present", path: targetPath, bytes: stats.size, reason: "" };
    }
    if (!stats.isDirectory()) {
      return { state: "present", path: targetPath, bytes: stats.size, reason: "" };
    }

    let total = stats.size;
    for (const entry of readdirSync(targetPath, { withFileTypes: true })) {
      const childPath = join(targetPath, entry.name);
      const child = measurePathWithNode(childPath);
      if (child.state === "present") {
        total += child.bytes;
      }
    }
    return { state: "present", path: targetPath, bytes: total, reason: "" };
  } catch (error) {
    return {
      state: "skipped",
      path: targetPath,
      bytes: 0,
      reason: error instanceof Error ? error.message : String(error)
    };
  }
}

function measureCurrentWorktree(rootPath) {
  const skippedNames = new Set([".git", ".worktrees"]);
  let total = 0;
  const skipped = [];

  for (const entry of safeReadDir(rootPath)) {
    if (skippedNames.has(entry.name)) {
      continue;
    }
    const entryPath = join(rootPath, entry.name);
    const measured = measurePath(entryPath);
    if (measured.state === "present") {
      total += measured.bytes;
    } else if (measured.state === "skipped") {
      skipped.push(measured);
    }
  }

  return { state: "present", path: rootPath, bytes: total, skipped };
}

function safeReadDir(targetPath) {
  try {
    return readdirSync(targetPath, { withFileTypes: true });
  } catch {
    return [];
  }
}

function formatBytes(bytes) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  if (unitIndex === 0) {
    return `${value} ${units[unitIndex]}`;
  }

  const precision = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
}

function relativeLabel(targetPath, rootPath = repoRoot) {
  const label = relative(rootPath, targetPath);
  return label && !label.startsWith("..") ? label : targetPath;
}

function collectPresentMeasurements(paths) {
  return paths
    .map((targetPath) => measurePath(targetPath))
    .filter((measurement) => measurement.state === "present");
}

function collectGeneratedOutputs(rootPath) {
  const directOutputs = generatedOutputDirs.map((name) => join(rootPath, name));
  return collectPresentMeasurements([...directOutputs, ...findTsBuildInfo(rootPath)]);
}

function collectRootDependencyInstall(rootPath) {
  return collectPresentMeasurements([join(rootPath, "node_modules")]);
}

function collectWorkerTargets(rootPath) {
  const workersDir = join(rootPath, "workers");
  if (!pathExists(workersDir)) {
    return [];
  }

  const targetPaths = safeReadDir(workersDir)
    .filter((entry) => entry.isDirectory())
    .map((entry) => join(workersDir, entry.name, "target"));

  return collectPresentMeasurements(targetPaths);
}

function findTsBuildInfo(rootPath) {
  const matches = [];

  function walk(currentPath) {
    for (const entry of safeReadDir(currentPath)) {
      const entryPath = join(currentPath, entry.name);
      if (entry.isDirectory()) {
        if (!scanSkipDirs.has(entry.name)) {
          walk(entryPath);
        }
        continue;
      }
      if (entry.isFile() && entry.name.endsWith(".tsbuildinfo")) {
        matches.push(entryPath);
      }
    }
  }

  walk(rootPath);
  return matches;
}

function parseWorktrees(raw) {
  const worktrees = [];
  let current = null;

  for (const line of raw.split(/\r?\n/)) {
    if (!line.trim()) {
      if (current) {
        worktrees.push(current);
        current = null;
      }
      continue;
    }

    const [key, ...rest] = line.split(" ");
    const value = rest.join(" ");
    if (key === "worktree") {
      if (current) {
        worktrees.push(current);
      }
      current = { path: value, head: "", branch: "", detached: false };
    } else if (current && key === "HEAD") {
      current.head = value;
    } else if (current && key === "branch") {
      current.branch = value.replace(/^refs\/heads\//, "");
    } else if (current && key === "detached") {
      current.detached = true;
    }
  }

  if (current) {
    worktrees.push(current);
  }

  return worktrees;
}

function getWorktrees() {
  const result = run(gitBin, ["worktree", "list", "--porcelain"]);
  if (!result.ok) {
    return { ok: false, reason: result.reason || "git worktree list failed", worktrees: [] };
  }
  return { ok: true, reason: "", worktrees: parseWorktrees(result.stdout) };
}

function getWorktreeStatus(worktreePath) {
  const result = run(gitBin, ["-C", worktreePath, "status", "--short"], {
    cwd: worktreePath
  });
  if (!result.ok) {
    return { state: "skipped", detail: result.reason || "git status failed" };
  }

  const lines = result.stdout.split(/\r?\n/).filter(Boolean);
  if (lines.length === 0) {
    return { state: "clean", detail: "0 changed paths" };
  }

  return {
    state: "dirty",
    detail: `${lines.length} changed ${lines.length === 1 ? "path" : "paths"}`
  };
}

function collectWorktreeMajorOutputs(worktreePath) {
  return [
    ...collectRootDependencyInstall(worktreePath),
    ...collectGeneratedOutputs(worktreePath),
    ...collectWorkerTargets(worktreePath)
  ];
}

function namedGoCachePaths() {
  const configured = process.env.RADIANT_NAMED_GO_CACHES;
  if (configured) {
    return configured.split(delimiter).filter(Boolean);
  }
  return ["/tmp/radiant-go-cache", "/tmp/radiant-go-mod-cache"];
}

function globalBunCachePaths() {
  return uniquePaths([
    process.env.BUN_INSTALL_CACHE_DIR || "",
    join(process.env.HOME || homedir(), ".bun", "install", "cache")
  ]);
}

function globalGoCaches() {
  const result = run(goBin, ["env", "GOCACHE", "GOMODCACHE"], { timeoutMs: 10000 });
  if (!result.ok) {
    return { ok: false, reason: result.reason || "go env failed", caches: [] };
  }

  const [goCache, goModCache] = result.stdout.split(/\r?\n/).map((line) => line.trim());
  return {
    ok: true,
    reason: "",
    caches: [
      { label: "GOCACHE", path: goCache },
      { label: "GOMODCACHE", path: goModCache }
    ].filter((cache) => cache.path)
  };
}

function uniquePaths(paths) {
  const seen = new Set();
  const values = [];
  for (const value of paths) {
    if (!value || seen.has(value)) {
      continue;
    }
    seen.add(value);
    values.push(value);
  }
  return values;
}

function dockerSystemDf() {
  const result = run(dockerBin, ["--context", dockerContext, "system", "df"], {
    timeoutMs: 30000
  });

  if (!result.ok) {
    return {
      ok: false,
      reason: result.reason || "docker system df unavailable",
      stdout: "",
      stderr: result.stderr || ""
    };
  }

  return {
    ok: true,
    reason: "",
    stdout: result.stdout.trimEnd(),
    stderr: result.stderr || ""
  };
}

function printMeasurementList(measurements, options = {}) {
  const indent = options.indent || "  ";
  if (measurements.length === 0) {
    console.log(`${indent}- ${options.emptyText || "none found"}`);
    return;
  }

  const rootPath = options.rootPath || repoRoot;
  for (const measurement of measurements) {
    console.log(`${indent}- ${relativeLabel(measurement.path, rootPath)}: ${formatBytes(measurement.bytes)} (${measurement.path})`);
  }
}

function printExternalMeasurement(label, targetPath) {
  const measurement = measurePath(targetPath);
  if (measurement.state === "present") {
    console.log(`  - ${label}: ${formatBytes(measurement.bytes)} (${targetPath})`);
  } else {
    console.log(`  - ${label}: skipped (${measurement.reason}) (${targetPath})`);
  }
}

function main() {
  console.log("Radiant hygiene size audit");
  console.log("Read-only: reports storage only; no files, Git state, caches, or Docker objects are changed.");
  console.log("");

  console.log("Repo-local storage");
  console.log(`- Current worktree: ${repoRoot}`);
  const repoSize = measureCurrentWorktree(repoRoot);
  console.log(`  Size: ${formatBytes(repoSize.bytes)} (excluding .git and nested .worktrees)`);
  if (repoSize.skipped.length > 0) {
    for (const skipped of repoSize.skipped) {
      console.log(`  Skipped: ${relativeLabel(skipped.path)} (${skipped.reason})`);
    }
  }
  console.log("- Root dependency install");
  printMeasurementList(collectRootDependencyInstall(repoRoot), {
    emptyText: "root node_modules not present"
  });
  console.log("- Generated and build-output paths");
  printMeasurementList(collectGeneratedOutputs(repoRoot), {
    emptyText: "storybook-static, dist, generated, .local, and *.tsbuildinfo not present"
  });
  console.log("- Rust worker build output");
  printMeasurementList(collectWorkerTargets(repoRoot), {
    emptyText: "workers/*/target not present"
  });
  console.log("");

  console.log("Registered Git worktrees");
  const worktreeResult = getWorktrees();
  if (!worktreeResult.ok) {
    console.log(`- Skipped: ${worktreeResult.reason}`);
  } else if (worktreeResult.worktrees.length === 0) {
    console.log("- none reported by Git");
  } else {
    for (const worktree of worktreeResult.worktrees) {
      const branchLabel = worktree.detached ? "detached HEAD" : worktree.branch || "unknown branch";
      const status = getWorktreeStatus(worktree.path);
      const size = measurePath(worktree.path);
      console.log(`- ${worktree.path}`);
      console.log(`  Branch: ${branchLabel}`);
      console.log(`  State: ${status.state}${status.detail ? ` (${status.detail})` : ""}`);
      if (size.state === "present") {
        console.log(`  Total size: ${formatBytes(size.bytes)}`);
      } else {
        console.log(`  Total size: skipped (${size.reason})`);
      }
      console.log("  Major dependency/build outputs:");
      printMeasurementList(collectWorktreeMajorOutputs(worktree.path), {
        rootPath: worktree.path,
        emptyText: "none found",
        indent: "    "
      });
    }
  }
  console.log("");

  console.log("External toolchain caches");
  console.log("- Named Radiant Go caches");
  for (const cachePath of namedGoCachePaths()) {
    printExternalMeasurement(cachePath, cachePath);
  }
  console.log("- Global Bun cache");
  for (const cachePath of globalBunCachePaths()) {
    printExternalMeasurement("Bun install cache", cachePath);
  }
  console.log("- Global Go caches");
  const goCaches = globalGoCaches();
  if (!goCaches.ok) {
    console.log(`  - skipped (${goCaches.reason})`);
  } else {
    for (const cache of goCaches.caches) {
      printExternalMeasurement(cache.label, cache.path);
    }
  }
  console.log("");

  console.log("Docker/OrbStack storage");
  console.log(`- Context: ${dockerContext}`);
  const docker = dockerSystemDf();
  if (!docker.ok) {
    console.log(`- Skipped: ${docker.reason}`);
  } else if (!docker.stdout.trim()) {
    console.log("- docker system df returned no output");
  } else {
    for (const line of docker.stdout.split(/\r?\n/)) {
      console.log(`  ${line}`);
    }
  }
}

main();
