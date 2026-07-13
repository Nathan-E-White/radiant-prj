import fs from "node:fs";

const files = [
  "docs/design/simops-runtime-adapters.md",
  "docs/design/interface-control.md",
  "docs/verification/verification-plan.md",
  "docs/requirements/verification-matrix.md",
];
const documents = Object.fromEntries(
  files.map((file) => [file, fs.readFileSync(file, "utf8")]),
);
const problems = [];

const requiredByFile = {
  "docs/design/simops-runtime-adapters.md": [
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
  ],
  "docs/design/interface-control.md": [
    "Docker sync maps container existence",
    "client-go",
    "Kubernetes sync uses the same state set",
    "Gateway-Only Worker Ingest",
    "OpenTofu owns static Kubernetes substrate only",
  ],
  "docs/verification/verification-plan.md": [
    "simops:smoke:docker-orbstack",
    "simops:smoke:kind",
    "simops:tofu:preflight",
  ],
  "docs/requirements/verification-matrix.md": [
    "SIMOPS-RUNTIME-CLOSEOUT-001",
    "simops:runtime:closeout:check",
  ],
};

for (const [file, tokens] of Object.entries(requiredByFile)) {
  for (const token of tokens) {
    if (!documents[file].includes(token)) {
      problems.push(`${file} missing ${token}`);
    }
  }
}

if (Object.values(documents).some((document) =>
  document.includes("Kubernetes sync will use the same state set"))) {
  problems.push("Runtime closeout docs still describe implemented Kubernetes sync as future work");
}

if (problems.length) {
  console.error(problems.map((problem) => `- ${problem}`).join("\n"));
  process.exit(1);
}
console.log("SimOps runtime closeout documentation checks passed.");
