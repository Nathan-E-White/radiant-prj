import { existsSync, readFileSync } from "node:fs";

const policyPath = "docs/design/docker-orbstack-storage-policy.md";
const reportPath = "scripts/hygiene-size.mjs";
const cleanupPath = "scripts/docker-prune-hygiene.sh";
const policy = existsSync(policyPath) ? readFileSync(policyPath, "utf8") : "";
const report = existsSync(reportPath) ? readFileSync(reportPath, "utf8") : "";
const cleanup = existsSync(cleanupPath) ? readFileSync(cleanupPath, "utf8") : "";
const problems = [];

function requireContent(content, path, tokens) {
  if (!content) {
    problems.push(`Missing policy source: ${path}`);
    return;
  }
  for (const token of tokens) {
    if (!content.includes(token)) {
      problems.push(`${path} missing required content: ${token}`);
    }
  }
}

requireContent(policy, policyPath, [
  "Docker/OrbStack Storage Policy", "Images", "Build cache", "Containers", "Volumes",
  "TOTAL", "ACTIVE", "SIZE", "RECLAIMABLE", "Protected by default",
  "docker system prune", "docker volume prune", "Radiant-owned labels", "dry-run",
  "docker --context orbstack system df"
]);
requireContent(report, reportPath, [
  "dockerBin, [\"--context\", dockerContext, \"system\", \"df\"]",
  "Docker/OrbStack storage", "if (!docker.ok)"
]);
requireContent(cleanup, cleanupPath, [
  "--scope-label", "Docker cleanup requires --scope-label", "--filter", "--confirm-volumes"
]);

if (/docker(?: --context orbstack)? (?:system|image|builder|container|volume) prune/.test(report)) {
  problems.push(`${reportPath} must remain read-only; found a Docker cleanup command`);
}

for (const kind of ["image", "builder", "container", "volume"]) {
  const command = new RegExp(`run_docker ${kind} [^\\n]*--filter \\\"label=\\$\\{SCOPE_LABEL\\}\\\"`);
  if (!command.test(cleanup)) {
    problems.push(`${cleanupPath} must scope ${kind} cleanup with a Docker label filter`);
  }
}

if (problems.length > 0) {
  console.error("Docker/OrbStack storage policy check failed:");
  for (const problem of problems) console.error(`- ${problem}`);
  process.exit(1);
}

console.log("Docker/OrbStack storage policy checks passed.");
