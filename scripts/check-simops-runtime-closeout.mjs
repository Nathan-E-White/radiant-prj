import fs from "node:fs";

const files = [
  "docs/design/simops-runtime-adapters.md",
  "docs/design/interface-control.md",
  "docs/verification/verification-plan.md",
  "docs/requirements/verification-matrix.md",
];
const combined = files.map((file) => fs.readFileSync(file, "utf8")).join("\n");
const problems = [];

for (const token of [
  "RunConnectionProfile",
  "Docker SDK",
  "client-go",
  "SyncRun",
  "Gateway-Only Worker Ingest",
  "SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT",
  "simops:smoke:docker-orbstack",
  "simops:smoke:kind",
  "simops:tofu:preflight",
  "CRD/operator",
  "Argo",
  "Tekton",
  "host-facing Redpanda",
  "production hardening",
]) {
  if (!combined.includes(token)) problems.push(`Runtime closeout docs missing ${token}`);
}

if (combined.includes("Kubernetes sync will use the same state set")) {
  problems.push("Runtime closeout docs still describe implemented Kubernetes sync as future work");
}

if (problems.length) {
  console.error(problems.map((problem) => `- ${problem}`).join("\n"));
  process.exit(1);
}
console.log("SimOps runtime closeout documentation checks passed.");
