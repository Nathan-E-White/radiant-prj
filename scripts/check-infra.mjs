import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const requiredFiles = [
  "docker-compose.yml",
  "Dockerfile",
  "worker.Dockerfile",
  "infra/terraform/main.tf",
  "infra/terraform/variables.tf",
  "infra/terraform/outputs.tf",
  "infra/ansible/inventory.ini",
  "infra/ansible/site.yml",
  "infra/ansible/templates/scheduler.conf.j2",
  "infra/ansible/templates/worker.service.j2",
  "infra/ansible/templates/logrotate.conf.j2"
];
const problems = [];

for (const file of requiredFiles) {
  if (!existsSync(file)) {
    problems.push(`Missing ${file}`);
  }
}

if (existsSync("infra/terraform/main.tf")) {
  const terraform = readFileSync("infra/terraform/main.tf", "utf8");
  for (const token of ["scheduler", "worker_pool", "artifact_bucket", "monitoring_endpoint"]) {
    if (!terraform.includes(token)) {
      problems.push(`Terraform environment missing ${token}`);
    }
  }
}

if (existsSync("infra/ansible/site.yml")) {
  const ansible = readFileSync("infra/ansible/site.yml", "utf8");
  for (const token of ["kaleidos_compute", "module_inventory", "systemd", "logrotate"]) {
    if (!ansible.includes(token)) {
      problems.push(`Ansible baseline missing ${token}`);
    }
  }
}

if (problems.length) {
  console.error("Infrastructure check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

optionalCheck("terraform", ["-chdir=infra/terraform", "fmt", "-check"]);
optionalCheck("terraform", ["-chdir=infra/terraform", "validate"]);
optionalCheck("ansible-playbook", ["--syntax-check", "-i", "infra/ansible/inventory.ini", "infra/ansible/site.yml"]);

console.log("Infrastructure artifact check passed.");

function optionalCheck(command, args) {
  const version = spawnSync(command, ["--version"], { stdio: "ignore" });
  if (version.error || version.status !== 0) {
    console.log(`${command} not available; static artifact checks already passed.`);
    return;
  }

  const result = spawnSync(command, args, { stdio: "inherit" });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
