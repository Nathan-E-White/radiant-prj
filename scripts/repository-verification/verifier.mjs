import { access, readFile, readdir, stat } from "node:fs/promises";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { isDeepStrictEqual } from "node:util";
import { parse as parseYaml } from "yaml";

const schemaVersion = "radiant.repository-verification.v1";

export async function verifyRepository({ manifest, root = process.cwd(), run = runProcess, claimIds } = {}) {
  const manifestProblem = validateManifest(manifest);
  if (manifestProblem) {
    return {
      exitCode: 1,
      results: [failure("manifest.invalid", "verification manifest", schemaVersion, manifestProblem)],
    };
  }

  const selected = claimIds?.length
    ? manifest.claims.filter(({ id }) => claimIds.includes(id))
    : manifest.claims;
  const selectedIds = new Set(selected.map(({ id }) => id));
  const results = (claimIds ?? [])
    .filter((id) => !selectedIds.has(id))
    .map((id) => failure(id, "verification manifest", "requested claim exists", "claim is not present in the manifest"));

  for (const claim of selected) {
    results.push(await verifyClaim(claim, { root, run }));
  }

  results.sort((left, right) => left.claimId.localeCompare(right.claimId));
  return {
    exitCode: results.some(({ status }) => status === "fail") ? 1 : 0,
    results,
  };
}

export function formatVerificationReport(report) {
  const lines = report.results.flatMap((result) => [
    `[${result.status.toUpperCase()}] ${result.claimId}: ${result.title}`,
    `  evidence: ${result.evidenceSource}`,
    `  expected: ${result.expected}`,
    `  observed: ${result.observed}`,
  ]);
  const passed = report.results.filter(({ status }) => status === "pass").length;
  const skipped = report.results.filter(({ status }) => status === "skip").length;
  const failed = report.results.length - passed - skipped;
  lines.push(`Repository verification: ${passed} passed, ${skipped} skipped, ${failed} failed.`);
  return lines.join("\n");
}

async function verifyClaim(claim, context) {
  const { adapter } = claim.evidence;
  switch (adapter) {
    case "json":
      return verifyJson(claim, context);
    case "yaml":
      return verifyYaml(claim, context);
    case "files":
      return verifyFiles(claim, context);
    case "source-set":
      return verifySourceSet(claim, context);
    case "document":
      return verifyDocument(claim, context);
    case "command":
      return verifyCommand(claim, context);
    case "compose":
      return verifyCompose(claim, context);
    case "opentofu":
      return verifyOpenTofu(claim, context);
    default:
      return claimFailure(claim, `unsupported adapter: ${String(adapter)}`);
  }
}

async function verifyJson(claim, { root }) {
  const loaded = await readTextEvidence(root, claim.evidence.source);
  if (loaded.problem) return claimFailure(claim, loaded.problem);
  const parsed = parseJson(loaded.text, claim.evidence.source);
  if (parsed.problem) return claimFailure(claim, parsed.problem);
  return verifyAssertions(claim, parsed.value);
}

async function verifyYaml(claim, { root }) {
  const loaded = await readTextEvidence(root, claim.evidence.source);
  if (loaded.problem) return claimFailure(claim, loaded.problem);
  let value;
  try {
    value = parseYaml(loaded.text);
  } catch (error) {
    return claimFailure(claim, `invalid YAML in ${claim.evidence.source}: ${error.message}`);
  }
  return verifyAssertions(claim, value);
}

async function verifyFiles(claim, { root }) {
  const missing = [];
  for (const source of claim.evidence.sources ?? []) {
    try {
      await access(path.resolve(root, source));
    } catch {
      missing.push(source);
    }
  }
  if (missing.length) return claimFailure(claim, `missing inputs: ${missing.sort().join(", ")}`);
  return claimPass(claim, `${claim.evidence.sources?.length ?? 0} required inputs exist`);
}

async function verifySourceSet(claim, { root }) {
  const sourcePath = path.resolve(root, claim.evidence.source);
  let files;
  try {
    files = await collectFiles(sourcePath, new Set(claim.evidence.extensions ?? []));
  } catch (error) {
    return claimFailure(claim, error.code === "ENOENT" ? "input is missing" : error.message);
  }
  files = files.filter((file) => !(claim.evidence.excludeSuffixes ?? []).some((suffix) => file.endsWith(suffix)));
  const text = (await Promise.all(files.map((file) => readFile(file, "utf8")))).join("\n");
  for (const required of claim.requiredText ?? []) {
    if (!text.includes(required)) return claimFailure(claim, `source set missing required text: ${JSON.stringify(required)}`);
  }
  for (const pattern of claim.forbiddenPatterns ?? []) {
    if (new RegExp(pattern, "im").test(text)) return claimFailure(claim, `source set contains forbidden pattern: ${pattern}`);
  }
  return claimPass(claim, `${files.length} files satisfy ${claim.requiredText?.length ?? 0} required and ${claim.forbiddenPatterns?.length ?? 0} forbidden text invariants`);
}

async function verifyDocument(claim, { root }) {
  const loaded = await readTextEvidence(root, claim.evidence.source);
  if (loaded.problem) return claimFailure(claim, loaded.problem);
  for (const wording of claim.requiredText ?? []) {
    if (!loaded.text.includes(wording)) return claimFailure(claim, `missing required wording: ${JSON.stringify(wording)}`);
  }
  return claimPass(claim, `${claim.requiredText?.length ?? 0} required text invariants satisfied`);
}

async function verifyCommand(claim, { root, run }) {
  if (claim.evidence.whenEnvironment && !process.env[claim.evidence.whenEnvironment]) {
    return claimSkip(claim, `not run: ${claim.evidence.whenEnvironment} is not set`);
  }
  const [command, ...args] = claim.evidence.command ?? [];
  if (!command) return claimFailure(claim, "command is missing");
  const execution = await executeEvidenceCommand(claim, { root, run }, command, args);
  if (execution.failure) return execution.failure;
  const { result } = execution;
  const output = conciseOutput(result.stdout, result.stderr);
  if (result.status !== 0) return claimFailure(claim, `exit ${result.status ?? "unknown"}${output ? `: ${output}` : ""}`);
  return claimPass(claim, `exit 0${output ? `: ${output}` : ""}`);
}

async function verifyCompose(claim, { root, run }) {
  const execution = await executeEvidenceCommand(
    claim,
    { root, run },
    "docker",
    ["compose", "-f", claim.evidence.source, "config", "--format", "json"],
  );
  if (execution.failure) return execution.failure;
  const { result } = execution;
  if (result.status !== 0) return claimFailure(claim, `docker compose config exited ${result.status ?? "unknown"}: ${conciseOutput(result.stderr, result.stdout)}`);
  const parsed = parseJson(result.stdout, "docker compose config output");
  if (parsed.problem) return claimFailure(claim, parsed.problem);
  return verifyAssertions(claim, parsed.value);
}

async function verifyOpenTofu(claim, { root, run }) {
  const formatOnly = claim.evidence.mode === "format";
  const args = formatOnly
    ? [`-chdir=${claim.evidence.source}`, "fmt", "-check", "-recursive"]
    : [`-chdir=${claim.evidence.source}`, "validate", "-json"];
  const execution = await executeEvidenceCommand(claim, { root, run }, "tofu", args);
  if (execution.failure) return execution.failure;
  const { result } = execution;
  if (formatOnly) {
    if (result.status !== 0) return claimFailure(claim, `OpenTofu fmt exited ${result.status ?? "unknown"}: ${conciseOutput(result.stderr, result.stdout)}`);
    return claimPass(claim, "OpenTofu parsed every configuration file and reported canonical formatting");
  }
  const parsed = parseJson(result.stdout, "OpenTofu validate output");
  if (parsed.problem) return claimFailure(claim, parsed.problem);
  if (result.status !== 0 || parsed.value.valid !== true) {
    const diagnostics = (parsed.value.diagnostics ?? []).map(({ summary, detail }) => summary || detail).filter(Boolean).join("; ");
    return claimFailure(claim, `valid=${String(parsed.value.valid)}${diagnostics ? `: ${diagnostics}` : ""}`);
  }
  return claimPass(claim, "OpenTofu validate reported valid=true");
}

async function readTextEvidence(root, source) {
  try {
    return { text: await readFile(path.resolve(root, source), "utf8") };
  } catch (error) {
    return { problem: error.code === "ENOENT" ? "input is missing" : error.message };
  }
}

function parseJson(text, label) {
  try {
    return { value: JSON.parse(text) };
  } catch (error) {
    return { problem: `invalid JSON in ${label}: ${error.message}` };
  }
}

function verifyAssertions(claim, value) {
  const observations = [];
  for (const assertion of claim.assertions ?? []) {
    const observed = valueAtPath(value, assertion.path);
    if ("some" in assertion) {
      const matched = Array.isArray(observed) && observed.some((item) => matchesPartial(item, assertion.some));
      if (!matched) return claimFailure(claim, `${assertion.path} has no item matching ${JSON.stringify(assertion.some)}`);
      observations.push(`${assertion.path} contains ${JSON.stringify(assertion.some)}`);
    } else if (!isDeepStrictEqual(observed, assertion.equals)) {
      return claimFailure(claim, `${assertion.path} = ${JSON.stringify(observed)}`);
    } else {
      observations.push(`${assertion.path} = ${JSON.stringify(observed)}`);
    }
  }
  return claimPass(claim, observations.join("; ") || "structured evidence parsed successfully");
}

function valueAtPath(value, dottedPath) {
  return dottedPath.split(".").reduce((current, segment) => current?.[segment], value);
}

function matchesPartial(actual, expected) {
  if (Array.isArray(expected)) {
    return Array.isArray(actual) && expected.every((expectedItem) => actual.some((actualItem) => matchesPartial(actualItem, expectedItem)));
  }
  if (expected && typeof expected === "object") {
    return actual && typeof actual === "object"
      && Object.entries(expected).every(([key, value]) => matchesPartial(actual[key], value));
  }
  return isDeepStrictEqual(actual, expected);
}

function claimPass(claim, observed) {
  return result(claim, "pass", observed);
}

function claimFailure(claim, observed) {
  return result(claim, "fail", observed);
}

function claimSkip(claim, observed) {
  return result(claim, "skip", observed);
}

function result(claim, status, observed) {
  return {
    claimId: claim.id,
    title: claim.title,
    status,
    evidenceSource: claim.evidence.source,
    expected: claim.expected,
    observed,
  };
}

function failure(claimId, evidenceSource, expected, observed) {
  return { claimId, title: "Verification manifest is valid", status: "fail", evidenceSource, expected, observed };
}

function validateManifest(manifest) {
  if (!manifest || typeof manifest !== "object") return "manifest must be an object";
  if (manifest.schemaVersion !== schemaVersion) return `schemaVersion must equal ${schemaVersion}`;
  if (!Array.isArray(manifest.claims)) return "claims must be an array";
  const ids = new Set();
  for (const [index, claim] of manifest.claims.entries()) {
    if (!claim?.id || !claim.title || !claim.expected || !claim.evidence?.adapter || !claim.evidence?.source) {
      return `claims[${index}] is missing id, title, expected, or evidence`;
    }
    if (ids.has(claim.id)) return `duplicate claim id: ${claim.id}`;
    ids.add(claim.id);
  }
}

function conciseOutput(...values) {
  return values.join("\n").trim().split(/\r?\n/).filter(Boolean).slice(-3).join(" | ");
}

async function executeEvidenceCommand(claim, { root, run }, command, args) {
  const cwd = path.resolve(root, claim.evidence.cwd ?? ".");
  const result = await run(command, args, { cwd, env: { ...process.env, ...claim.evidence.env } });
  if (result.error?.code === "ENOENT") return { failure: claimFailure(claim, `tool not found: ${command}`) };
  if (result.error) return { failure: claimFailure(claim, result.error.message) };
  return { result };
}

function runProcess(command, args, options) {
  return spawnSync(command, args, { ...options, encoding: "utf8" });
}

async function collectFiles(sourcePath, extensions) {
  if ((await stat(sourcePath)).isFile()) return extensions.size === 0 || extensions.has(path.extname(sourcePath)) ? [sourcePath] : [];
  const entries = await readdir(sourcePath, { withFileTypes: true });
  const files = await Promise.all(entries.map((entry) => {
    const entryPath = path.join(sourcePath, entry.name);
    if (entry.isDirectory()) return collectFiles(entryPath, extensions);
    return extensions.size === 0 || extensions.has(path.extname(entry.name)) ? [entryPath] : [];
  }));
  return files.flat().sort();
}
