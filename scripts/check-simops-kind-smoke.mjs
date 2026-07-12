import fs from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const smokePath = path.join(root, "scripts", "simops-kind-smoke.sh");
const problems = [];

if (!fs.existsSync(smokePath)) {
  problems.push("scripts/simops-kind-smoke.sh is missing");
} else {
  const smoke = fs.readFileSync(smokePath, "utf8");
  for (const required of [
    "kind create cluster",
    "kind load docker-image",
    "SIMOPS_WORKER_RUNTIME",
    "kubernetes",
    "simops.run_id",
    "simops.worker_id",
    "simops.worker_kind",
    "wait_for_state \"$success_run\" succeeded --frames",
    "image-pull-failed",
    "ttlSecondsAfterFinished",
    "delete job \"$success_job\"",
    "cluster_context=",
    "namespace=",
    "job_name=",
    "run_id=",
    "final_lifecycle=",
  ]) {
    if (!smoke.includes(required)) {
      problems.push(`Kind smoke is missing required evidence token: ${required}`);
    }
  }
  if (!smoke.includes("DOCKER_CONTEXT=\"${SIMOPS_DOCKER_CONTEXT:-orbstack}\"")) {
    problems.push("Kind smoke must scope Docker to the orbstack context by default");
  }
  if (!smoke.includes("SIMOPS_KIND_FORCE_CLEANUP")) {
    problems.push("Kind smoke must expose an explicit force-cleanup policy");
  }
}

if (problems.length > 0) {
  console.error(problems.map((problem) => `- ${problem}`).join("\n"));
  process.exit(1);
}

console.log("Kind SimOps smoke contract checks passed.");
