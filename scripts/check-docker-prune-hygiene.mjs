import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url)).replace(/\/$/, "");
const scriptPath = join(repoRoot, "scripts", "docker-prune-hygiene.sh");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function runScenario(args) {
  const tempDir = mkdtempSync(join(tmpdir(), "radiant-docker-prune-test-"));
  const logPath = join(tempDir, "docker.log");
  const fakeDocker = join(tempDir, "docker");

  writeFileSync(
    fakeDocker,
    `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "${logPath}"
echo "fake docker $*"
`
  );

  spawnSync("chmod", ["755", fakeDocker], { encoding: "utf8" });

  const result = spawnSync("bash", [scriptPath, ...args], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      DOCKER_BIN: fakeDocker
    }
  });

  let dockerLog = "";
  try {
    dockerLog = readFileSync(logPath, "utf8");
  } catch {
    dockerLog = "";
  }

  rmSync(tempDir, { recursive: true, force: true });

  return {
    status: result.status,
    stdout: result.stdout,
    stderr: result.stderr,
    dockerLog,
    dockerBin: fakeDocker
  };
}

function assertIncludes(value, expected, message) {
  assert(value.includes(expected), message);
}

function assertScenario({ name, args, status, stdout = [], stderr = [], dockerLog = undefined }) {
  const result = runScenario(args);
  assert(result.status === status, `${name} exited ${result.status}, expected ${status}: ${result.stderr}`);
  for (const expected of stdout) {
    assertIncludes(result.stdout, expected.replaceAll("{dockerBin}", result.dockerBin), `${name} stdout missing ${expected}`);
  }
  for (const expected of stderr) {
    assertIncludes(result.stderr, expected, `${name} stderr missing ${expected}`);
  }
  if (dockerLog !== undefined) {
    assert(result.dockerLog === dockerLog, `${name} docker log mismatch: ${result.dockerLog}`);
  }
  return result;
}

const dryRun = runScenario(["--all"]);
assert(dryRun.status === 0, `dry-run should pass: ${dryRun.stderr}`);
assertIncludes(dryRun.stdout, "Dry run: no Docker prune command was executed.", "dry-run should be explicit");
assertIncludes(dryRun.stdout, `${dryRun.dockerBin} --context orbstack image prune --all --force`, "dry-run should plan image pruning");
assertIncludes(dryRun.stdout, `${dryRun.dockerBin} --context orbstack builder prune --force`, "dry-run should plan build-cache pruning");
assertIncludes(dryRun.stdout, `${dryRun.dockerBin} --context orbstack container prune --force`, "dry-run should plan stopped-container pruning");
assertIncludes(dryRun.stdout, `${dryRun.dockerBin} --context orbstack volume prune --force`, "dry-run --all should plan volume pruning");
assert(dryRun.dockerLog === "", "dry-run must not call Docker");

for (const scenario of [
  {
    name: "missing category",
    args: [],
    status: 2,
    stderr: ["Select at least one prune category"],
    dockerLog: ""
  },
  {
    name: "unconfirmed volume execute",
    args: ["--volumes", "--execute"],
    status: 2,
    stderr: ["--confirm-volumes"],
    dockerLog: ""
  },
  {
    name: "missing context value",
    args: ["--images", "--context"],
    status: 2,
    stderr: ["--context requires a non-empty value"],
    dockerLog: ""
  }
]) {
  assertScenario(scenario);
}

const executeResult = runScenario(["--images", "--build-cache", "--containers", "--execute"]);
assert(executeResult.status === 0, `execute with fake Docker should pass: ${executeResult.stderr}`);
assert(executeResult.dockerLog.includes("--context orbstack system df"), "execute should capture Docker storage before pruning");
assert(executeResult.dockerLog.includes("--context orbstack image prune --all --force"), "execute should call image prune");
assert(executeResult.dockerLog.includes("--context orbstack builder prune --force"), "execute should call builder prune");
assert(executeResult.dockerLog.includes("--context orbstack container prune --force"), "execute should call container prune");

const volumeExecute = runScenario(["--volumes", "--confirm-volumes", "--execute"]);
assert(volumeExecute.status === 0, `confirmed volume execute with fake Docker should pass: ${volumeExecute.stderr}`);
assert(volumeExecute.dockerLog.includes("--context orbstack volume prune --force"), "confirmed volume execute should call volume prune");

console.log("Docker prune hygiene script checks passed.");
