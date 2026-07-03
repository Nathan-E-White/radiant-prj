import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const requiredFiles = [
  "docker-compose.yml",
  "Dockerfile",
  "worker.Dockerfile",
  "backend/slurm-gateway/go.mod",
  "backend/slurm-gateway/cmd/server/main.go",
  "backend/slurm-gateway/internal/gateway/handlers.go",
  "deploy/slurm-gateway.Dockerfile",
  "deploy/slurm-gateway.compose.yml",
  "deploy/prometheus.yml",
  "scripts/create-local-gateway-certs.sh",
  "infra/terraform/main.tf",
  "infra/terraform/variables.tf",
  "infra/terraform/outputs.tf",
  "infra/terraform/vault.sh",
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

if (existsSync("deploy/slurm-gateway.Dockerfile")) {
  const dockerfile = readFileSync("deploy/slurm-gateway.Dockerfile", "utf8");
  for (const token of ["backend/slurm-gateway", "go test ./...", "USER appuser", "SLURM_GATEWAY_MODE=mock"]) {
    if (!dockerfile.includes(token)) {
      problems.push(`Slurm gateway Dockerfile missing ${token}`);
    }
  }
}

if (existsSync("deploy/slurm-gateway.compose.yml")) {
  const compose = readFileSync("deploy/slurm-gateway.compose.yml", "utf8");
  for (const token of ["slurm-gateway", "SLURM_GATEWAY_ALLOWED_CLIENTS", "no-new-privileges:true", "prometheus"]) {
    if (!compose.includes(token)) {
      problems.push(`Slurm gateway compose missing ${token}`);
    }
  }
}

if (existsSync("deploy/prometheus.yml")) {
  const prometheus = readFileSync("deploy/prometheus.yml", "utf8");
  if (!prometheus.includes("slurm-gateway:8080")) {
    problems.push("Prometheus config must scrape slurm-gateway:8080");
  }
}

if (existsSync("scripts/create-local-gateway-certs.sh")) {
  const certScript = readFileSync("scripts/create-local-gateway-certs.sh", "utf8");
  for (const token of [".local/certs", "client-authorized", "client-unauthorized", "subjectAltName"]) {
    if (!certScript.includes(token)) {
      problems.push(`Local gateway cert helper missing ${token}`);
    }
  }
}

if (existsSync("infra/terraform/vault.sh")) {
  const vault = readFileSync("infra/terraform/vault.sh", "utf8");
  for (const forbidden of ["root_dev_token", "allow_any_name=true", "VAULT_DEV_ROOT_TOKEN_ID"]) {
    if (vault.includes(forbidden)) {
      problems.push(`Vault helper contains forbidden development secret/config token ${forbidden}`);
    }
  }
  for (const required of ["VAULT_ADDR", "VAULT_TOKEN", ".local/vault", "client_flag=true"]) {
    if (!vault.includes(required)) {
      problems.push(`Vault helper missing ${required}`);
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
