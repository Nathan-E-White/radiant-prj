import fs from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const moduleDir = path.join(root, "infra", "opentofu", "simops-kind-substrate");
const preflightPath = path.join(root, "scripts", "simops-opentofu-preflight.sh");
const problems = [];

for (const file of [".terraform.lock.hcl", "versions.tf", "variables.tf", "main.tf", "outputs.tf", "README.md"]) {
  if (!fs.existsSync(path.join(moduleDir, file))) problems.push(`OpenTofu substrate module missing ${file}`);
}

if (fs.existsSync(path.join(moduleDir, "main.tf"))) {
  const source = fs.readFileSync(path.join(moduleDir, "main.tf"), "utf8");
  for (const token of ["kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_role_v1", "kubernetes_role_binding_v1", "kubernetes_config_map_v1"]) {
    if (!source.includes(token)) problems.push(`OpenTofu substrate missing ${token}`);
  }
  if (/resource\s+"kubernetes_job/i.test(source)) problems.push("OpenTofu substrate must not own per-run Kubernetes Jobs");
}

if (!fs.existsSync(preflightPath)) {
  problems.push("scripts/simops-opentofu-preflight.sh is missing");
} else {
  const preflight = fs.readFileSync(preflightPath, "utf8");
  for (const token of ["tofu fmt -check", "tofu init -backend=false", "tofu validate", "tofu plan", "-refresh=false", "plan_summary=", "namespace=", "service_account="]) {
    if (!preflight.includes(token)) problems.push(`OpenTofu preflight missing ${token}`);
  }
  if (/\btofu\s+apply\b/.test(preflight)) problems.push("OpenTofu preflight must not apply infrastructure");
}

if (problems.length) {
  console.error(problems.map((problem) => `- ${problem}`).join("\n"));
  process.exit(1);
}
console.log("OpenTofu SimOps substrate contract checks passed.");
