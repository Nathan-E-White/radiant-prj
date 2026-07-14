import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const requiredFiles = [
  "docker-compose.yml",
  "Dockerfile",
  "worker.Dockerfile",
  ".github/workflows/ci.yml",
  "backend/slurm-gateway/go.mod",
  "backend/slurm-gateway/cmd/server/main.go",
  "backend/slurm-gateway/internal/gateway/handlers.go",
  "deploy/slurm-gateway.Dockerfile",
  "deploy/slurm-gateway.compose.yml",
  "deploy/simops-generator.Dockerfile",
  "deploy/postgres-init/001_simops.sql",
  "deploy/prometheus.yml",
  "scripts/simops-smoke-json.mjs",
  "scripts/simops-smoke-json.node-test.mjs",
  "scripts/simops-docker-orbstack-smoke.sh",
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
  for (const token of [
    "ARG SIMOPS_GO_BUILDER_IMAGE",
    "ARG SIMOPS_GATEWAY_RUNTIME_IMAGE",
    "backend/slurm-gateway",
    "go test -tags dataplane,iceberggo ./...",
    "go build -tags dataplane,iceberggo -trimpath -ldflags=\"-s -w\" -o /out/slurm-gateway ./cmd/server",
    "go build -tags dataplane -trimpath -ldflags=\"-s -w\" -o /out/simops-stream-gateway ./cmd/simops-stream-gateway",
    "-tags dataplane",
    "-tags dataplane,iceberggo",
    "simops-stream-gateway",
    "simops-timescale-writer",
    "simops-iceberg-writer",
    "simops-webtransport-probe",
    "docker-cli",
    "USER appuser",
    "SLURM_GATEWAY_MODE=mock",
    "CMD [\"/app/slurm-gateway\"]"
  ]) {
    if (!dockerfile.includes(token)) {
      problems.push(`Slurm gateway Dockerfile missing ${token}`);
    }
  }
}

if (existsSync("deploy/slurm-gateway.compose.yml")) {
  const compose = readFileSync("deploy/slurm-gateway.compose.yml", "utf8");
  for (const token of [
    "slurm-gateway",
    "SIMOPS_CONTROL_STORE",
    "SIMOPS_TELEMETRY_LOG",
    "SIMOPS_WORKER_RUNTIME",
    "SIMOPS_WORKER_INGEST_BASE_URL",
    "SIMOPS_WORKER_NETWORK",
    "SIMOPS_WORKER_KUBERNETES_NAMESPACE",
    "SIMOPS_WORKER_FRAME_OVERRIDE",
    "SIMOPS_WORKER_CLEANUP_TTL",
    "SIMOPS_WORKER_AUTO_REMOVE",
    "SIMOPS_GO_BUILDER_IMAGE",
    "SIMOPS_GATEWAY_RUNTIME_IMAGE",
    "SIMOPS_RUST_BUILDER_IMAGE",
    "SIMOPS_GENERATOR_RUNTIME_IMAGE",
    "SIMOPS_REDPANDA_IMAGE",
    "SIMOPS_TIMESCALE_IMAGE",
    "SIMOPS_MINIO_IMAGE",
    "SIMOPS_MINIO_MC_IMAGE",
    "SIMOPS_PROMETHEUS_IMAGE",
    "SIMOPS_MOQ_WEBTRANSPORT_URL",
    "SIMOPS_ICEBERG_WRITER_MODE",
    "iceberg-go",
    "SIMOPS_TIMESCALE_CONSUMER_GROUP",
    "SIMOPS_ICEBERG_CONSUMER_GROUP",
    "SIMOPS_MOQ_CONSUMER_GROUP",
    "SIMOPS_ICEBERG_BATCH_SIZE",
    "SIMOPS_ICEBERG_FLUSH_INTERVAL",
    "SIMOPS_ICEBERG_S3_ACCESS_KEY_ID",
    "SIMOPS_ICEBERG_S3_SECRET_ACCESS_KEY",
    "AWS_REGION",
    "AWS_DEFAULT_REGION",
    "AWS_S3_ENDPOINT",
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_EC2_METADATA_DISABLED",
    "simops-moq-gateway",
    "simops-stream-gateway",
    "SIMOPS_STREAM_GATEWAY_TLS_CERT_FILE",
    "SIMOPS_STREAM_GATEWAY_TLS_KEY_FILE",
    ".local/compose-secrets",
    "simops_moq_gateway_ca_crt",
    "/run/secrets/simops_moq_gateway_server_crt",
    "/run/secrets/simops_moq_gateway_server_key",
    "simops-timescale-writer",
    "simops-iceberg-writer",
    "http://slurm-gateway:8080",
    "http://127.0.0.1:9443/healthz",
    "http://127.0.0.1:9450/healthz",
    "http://127.0.0.1:9460/healthz",
    "redpanda",
    "postgres",
    "timescale/timescaledb",
    "minio",
    "simops-bucket-scheduler",
    "radiant-simops-generator:latest",
    "radiant-simops-local",
    "/var/run/docker.sock",
    "SLURM_GATEWAY_ALLOWED_CLIENTS",
    "no-new-privileges:true",
    "prometheus"
  ]) {
    if (!compose.includes(token)) {
      problems.push(`Slurm gateway compose missing ${token}`);
    }
  }
}

if (existsSync("scripts/simops-local-smoke.sh")) {
  const smoke = readFileSync("scripts/simops-local-smoke.sh", "utf8");
  for (const token of ["--ca-cert", "/run/secrets/simops_moq_gateway_ca_crt", "--server-name", "simops-moq-gateway"]) {
    if (!smoke.includes(token)) {
      problems.push(`SimOps local smoke missing WebTransport TLS probe token ${token}`);
    }
  }
}

if (existsSync(".github/workflows/ci.yml")) {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  if (!workflow.includes("permissions:\n  contents: read")) {
    problems.push("GitHub Actions workflow must limit GITHUB_TOKEN to contents: read");
  }
}

if (existsSync("deploy/simops-generator.Dockerfile")) {
  const dockerfile = readFileSync("deploy/simops-generator.Dockerfile", "utf8");
  for (const token of [
    "ARG SIMOPS_RUST_BUILDER_IMAGE",
    "ARG SIMOPS_GENERATOR_RUNTIME_IMAGE",
    "workers/simops-generator",
    "cargo test --locked",
    "gcr.io/distroless/static-debian13:nonroot",
    "/examples/simulation-ops",
    "ENTRYPOINT [\"/simops-generator\"]"
  ]) {
    if (!dockerfile.includes(token)) {
      problems.push(`Simulation Ops generator Dockerfile missing ${token}`);
    }
  }
}

if (existsSync("deploy/prometheus.yml")) {
  const prometheus = readFileSync("deploy/prometheus.yml", "utf8");
  if (!prometheus.includes("slurm-gateway:8080")) {
    problems.push("Prometheus config must scrape slurm-gateway:8080");
  }
  for (const token of ["simops-moq-gateway:9443", "simops-timescale-writer:9450", "simops-iceberg-writer:9460", "redpanda:9644"]) {
    if (!prometheus.includes(token)) {
      problems.push(`Prometheus config missing ${token}`);
    }
  }
}

if (existsSync("deploy/postgres-init/001_simops.sql")) {
  const sql = readFileSync("deploy/postgres-init/001_simops.sql", "utf8");
  for (const token of ["timescaledb", "create_hypertable", "simops_runs", "ingest_token", "simops_workers", "simops_events", "simops_artifacts", "simops_telemetry_frames", "simops_consumer_offsets", "simops_processed_messages", "workbench_twin_publications", "iceberg_catalog", "iceberg_tables", "iceberg_namespace_properties"]) {
    if (!sql.includes(token)) {
      problems.push(`SimOps Postgres init SQL missing ${token}`);
    }
  }
}

if (existsSync("scripts/create-local-gateway-certs.sh")) {
  const certScript = readFileSync("scripts/create-local-gateway-certs.sh", "utf8");
  for (const token of [".local/certs", ".local/compose-secrets", "client-authorized", "client-unauthorized", "subjectAltName", "simops-moq-gateway"]) {
    if (!certScript.includes(token)) {
      problems.push(`Local gateway cert helper missing ${token}`);
    }
  }
}

for (const sourceFile of [
  "backend/slurm-gateway/cmd/simops-webtransport-probe/main.go",
  "backend/slurm-gateway/cmd/simops-stream-gateway/main.go",
  "backend/slurm-gateway/internal/gateway/webtransport_tls.go"
]) {
  if (existsSync(sourceFile) && readFileSync(sourceFile, "utf8").includes("InsecureSkipVerify")) {
    problems.push(`${sourceFile} must not disable TLS certificate verification`);
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
  const version = trySpawn(command, ["--version"], { stdio: "ignore" });
  if (version.error || version.status !== 0) {
    console.log(`${command} not available; static artifact checks already passed.`);
    return;
  }

  const result = trySpawn(command, args, { stdio: "inherit" });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

function trySpawn(command, args, options) {
  try {
    return spawnSync(command, args, options);
  } catch (error) {
    return { error, status: null };
  }
}
