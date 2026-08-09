#!/usr/bin/env node

import { spawn, spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

const root = resolve(dirname(new URL(import.meta.url).pathname), "..");
const defaultManifest = "examples/chaos/simops-timescale-writer-pause.json";

function fail(message) {
  throw new Error(message);
}

function parseArgs(args) {
  const [command, ...rest] = args;
  const options = { execute: false, manifest: defaultManifest };
  for (let index = 0; index < rest.length; index += 1) {
    if (rest[index] === "--execute") options.execute = true;
    else if (rest[index] === "--manifest") options.manifest = rest[++index] ?? "";
    else fail(`Unknown argument: ${rest[index]}`);
  }
  return { command, options };
}

function loadManifest(manifestPath) {
  let manifest;
  try {
    manifest = JSON.parse(readFileSync(resolve(root, manifestPath), "utf8"));
  } catch (error) {
    fail(`Could not read chaos manifest ${manifestPath}: ${error.message}`);
  }
  const required = ["version", "id", "title", "hypothesis", "environment", "dockerContext", "composeFile", "composeProject", "service", "requiredLabel", "fault", "limits", "probes"];
  for (const key of required) if (manifest[key] === undefined) fail(`Manifest is missing ${key}.`);
  if (manifest.version !== 1) fail("Manifest version must be 1.");
  if (manifest.environment !== "local-orbstack" || manifest.dockerContext !== "orbstack") fail("Only local-orbstack experiments on the orbstack Docker context are allowed.");
  if (manifest.fault?.kind !== "container-pause") fail("This runner supports only the bounded container-pause fault.");
  if (manifest.fault.framework?.name !== "pumba" || !String(manifest.fault.framework.image).startsWith("ghcr.io/alexei-led/pumba:")) fail("The container-pause fault must use a versioned Pumba image.");
  if (!Number.isInteger(manifest.fault.durationSeconds) || manifest.fault.durationSeconds < 1 || manifest.fault.durationSeconds > 30) fail("fault.durationSeconds must be an integer from 1 to 30.");
  for (const key of ["totalSeconds", "probeSeconds", "recoverySeconds"]) if (!Number.isInteger(manifest.limits?.[key]) || manifest.limits[key] < 1) fail(`limits.${key} must be a positive integer.`);
  if (manifest.limits.totalSeconds < manifest.fault.durationSeconds + manifest.limits.recoverySeconds) fail("limits.totalSeconds must cover the fault and recovery windows.");
  for (const key of ["baseline", "duringFault", "recovery"]) if (!String(manifest.probes?.[key] ?? "").startsWith("http://127.0.0.1:")) fail(`probes.${key} must be an explicit loopback HTTP URL.`);
  if (manifest.requiredLabel !== "com.radiant.chaos-enabled=true") fail("requiredLabel must be com.radiant.chaos-enabled=true.");
  return manifest;
}

function docker(manifest, args, { capture = false, allowFailure = false } = {}) {
  const result = spawnSync("docker", ["--context", manifest.dockerContext, ...args], { cwd: root, encoding: "utf8" });
  if (result.error) fail(`Could not invoke Docker: ${result.error.message}`);
  if (result.status !== 0 && !allowFailure) fail((result.stderr || result.stdout || "Docker command failed.").trim());
  return capture ? (result.stdout || "").trim() : result.status === 0;
}

function compose(manifest, args) {
  return docker(manifest, ["compose", "-p", manifest.composeProject, "-f", manifest.composeFile, ...args]);
}

function targetId(manifest) {
  const ids = docker(manifest, ["ps", "-aq", "--filter", `label=com.docker.compose.project=${manifest.composeProject}`, "--filter", `label=com.docker.compose.service=${manifest.service}`, "--filter", "label=com.radiant.chaos-enabled=true"], { capture: true });
  const matches = ids.split("\n").filter(Boolean);
  if (matches.length !== 1) fail(`Expected exactly one opted-in ${manifest.service} container in Compose project ${manifest.composeProject}; found ${matches.length}. Run chaos:up first and do not broaden the selector.`);
  return matches[0];
}

function probe(url, timeoutSeconds) {
  const result = spawnSync("curl", ["--fail", "--silent", "--show-error", "--max-time", String(timeoutSeconds), url], { encoding: "utf8" });
  return { ok: result.status === 0, status: result.status, output: (result.stdout || result.stderr || "").trim() };
}

function waitFor(url, timeoutSeconds, expected) {
  const until = Date.now() + timeoutSeconds * 1000;
  let observation;
  do {
    observation = probe(url, Math.min(2, timeoutSeconds));
    if (observation.ok === expected) return observation;
  } while (Date.now() < until);
  return observation;
}

function evidencePath(id) {
  const stamp = new Date().toISOString().replaceAll(":", "-").replaceAll(".", "-");
  return resolve(root, ".local", "chaos-evidence", `${stamp}-${id}.json`);
}

function writeEvidence(path, evidence) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(evidence, null, 2)}\n`);
  console.log(`Evidence: ${path}`);
}

function pumbaPause(manifest, containerId) {
  const args = ["--context", manifest.dockerContext, "run", "--rm", "-v", "/var/run/docker.sock:/var/run/docker.sock", manifest.fault.framework.image, "pause", "--duration", `${manifest.fault.durationSeconds}s`, containerId];
  const child = spawn("docker", args, { cwd: root, stdio: ["ignore", "pipe", "pipe"] });
  let output = "";
  let settled = false;
  child.stdout.on("data", (chunk) => { output += chunk; });
  child.stderr.on("data", (chunk) => { output += chunk; });
  const done = new Promise((resolve, reject) => {
    const hardLimit = setTimeout(() => {
      if (!settled) {
        child.kill("SIGTERM");
        settled = true;
        reject(new Error(`Pumba exceeded its ${manifest.fault.durationSeconds + manifest.limits.probeSeconds}s independent cleanup limit.`));
      }
    }, (manifest.fault.durationSeconds + manifest.limits.probeSeconds) * 1000);
    child.on("error", reject);
    child.on("close", (status) => {
      if (settled) return;
      settled = true;
      clearTimeout(hardLimit);
      status === 0 ? resolve(output.trim()) : reject(new Error(output.trim() || `Pumba exited with status ${status}.`));
    });
  });
  return { done, terminate: () => child.kill("SIGTERM") };
}

function printPlan(manifest) {
  console.log(JSON.stringify({
    id: manifest.id,
    hypothesis: manifest.hypothesis,
    scope: { dockerContext: manifest.dockerContext, composeProject: manifest.composeProject, service: manifest.service, requiredLabel: manifest.requiredLabel },
    fault: manifest.fault,
    probes: manifest.probes,
    recovery: "docker unpause of the same labelled container; no container or volume deletion",
    execution: "append --execute to mutate local OrbStack state"
  }, null, 2));
}

async function runExperiment(manifest) {
  const evidence = { manifest, startedAt: new Date().toISOString(), observations: {}, rollback: { attempted: false, recovered: false }, verdict: "failed" };
  const path = evidencePath(manifest.id);
  let containerId;
  let pumba;
  try {
    containerId = targetId(manifest);
    evidence.containerId = containerId;
    evidence.observations.baseline = probe(manifest.probes.baseline, manifest.limits.probeSeconds);
    if (!evidence.observations.baseline.ok) fail("Baseline probe failed; no fault was injected.");
    pumba = pumbaPause(manifest, containerId);
    evidence.observations.duringFault = waitFor(manifest.probes.duringFault, manifest.limits.probeSeconds, false);
    if (evidence.observations.duringFault.ok) fail("During-fault probe remained healthy; the intended suspension was not observed.");
    evidence.framework = { name: manifest.fault.framework.name, output: await pumba.done };
  } catch (error) {
    evidence.error = error.message;
  } finally {
    if (pumba) {
      pumba.terminate();
      try {
        await pumba.done;
      } catch (error) {
        evidence.framework = { name: manifest.fault.framework.name, error: error.message };
      }
    }
    if (containerId) {
      evidence.rollback.attempted = true;
      try {
        docker(manifest, ["unpause", containerId], { allowFailure: true });
        evidence.observations.recovery = waitFor(manifest.probes.recovery, manifest.limits.recoverySeconds, true);
        evidence.rollback.recovered = evidence.observations.recovery.ok;
      } catch (error) {
        evidence.rollback.error = error.message;
      }
    }
    evidence.finishedAt = new Date().toISOString();
    evidence.verdict = !evidence.error && evidence.observations.baseline?.ok && !evidence.observations.duringFault?.ok && evidence.rollback.recovered ? "passed" : "failed";
    writeEvidence(path, evidence);
  }
  if (evidence.verdict !== "passed") fail(`Experiment failed. Rollback recovered: ${evidence.rollback.recovered}.`);
  console.log("Experiment passed: baseline healthy, pause observed, and recovery healthy.");
}

async function main() {
  const { command, options } = parseArgs(process.argv.slice(2));
  if (command === "--help" || command === "help" || !command) {
    console.log("Usage: chaos-orchestrate.mjs <plan|up|run|status|down> [--manifest path] [--execute]");
    return;
  }
  const manifest = loadManifest(options.manifest);
  if (command === "plan") return printPlan(manifest);
  if (command === "status") return docker(manifest, ["ps", "--filter", `label=com.docker.compose.project=${manifest.composeProject}`, "--filter", `label=com.docker.compose.service=${manifest.service}`, "--format", "table {{.ID}}\\t{{.Status}}\\t{{.Names}}"]);
  if (!options.execute) fail(`${command} is read-only until you append --execute.`);
  if (command === "up") return compose(manifest, ["up", "-d", manifest.service]);
  if (command === "down") return compose(manifest, ["stop", manifest.service]);
  if (command === "run") return await runExperiment(manifest);
  fail(`Unknown command: ${command}`);
}

try {
  await main();
} catch (error) {
  console.error(`chaos: ${error.message}`);
  process.exitCode = 1;
}
