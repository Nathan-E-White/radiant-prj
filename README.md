# Kaleidos Compute Readiness Console

Interview-facing engineering console for a Radiant application packet. The app combines source-linked public facts about Kaleidos with synthetic compute jobs, HPC failure triage, DevOps deployment artifacts, and a controlled requirements/evidence framework.

This is public-safe demo material. It is not reactor design, safety analysis, licensing evidence, or proprietary Radiant work.

## What It Shows

- **Kaleidos Brief:** public-source product facts, a stylized cutaway, readiness milestones, and explicit claim boundaries.
- **Compute Workbench:** deterministic toy transport, thermal, fleet, and infrastructure jobs with scheduler states, logs, artifacts, and diagnosis.
- **Evidence Matrix:** requirements, verification methods, evidence packs, artifact hashes, deployment checks, and controlled change notes.
- **DevOps Layer:** Docker Compose, dry-run Terraform, Ansible baseline templates, and CI checks for the synthetic hybrid compute environment.
- **Version 3.0 Backend Gateway:** mock-first Go Slurm gateway handlers with mTLS identity checks, request validation, job status lookup, and Prometheus-format metrics.
- **Simulation Ops Backend Slice:** Go run control endpoints, MoQ/WebTransport subscription metadata, Redpanda/Postgres/MinIO/Iceberg deployment seams, and Rust telemetry bucket containers.
- **Simulation Ops Contract:** telemetry schemas, NDJSON examples, MoQ/WebTransport live-track names, and scenario randomization blueprint for local worker swarms.
- **Controlled Quality Package:** controlled quality, design, verification, release, records documentation, and fixture-backed process evidence suitable for serious engineering review.

## Run Locally

```bash
bun install
bun run dev
```

The Vite dev server prints the local URL, typically `http://127.0.0.1:5173/`.

## Verification

```bash
bun run typecheck
bun run test
bun run validate:fixtures
bun run evidence:generate
bun run infra:check
bun run quality:check
bun run backend:test
bun run simops:contract:check
```

`bun run ci` runs the full local verification chain.

## Version 3.0 Backend Gateway

The v3.0 backend lives under `backend/slurm-gateway`. It exposes `GET /healthz`, `GET /readyz`, `GET /metrics`, `POST /api/jobs/submit`, and `GET /api/jobs/{job_id}`.

Default mode is `mock`, which returns deterministic synthetic Slurm job IDs and keeps the demo public-safe. Real `sbatch` submission is opt-in through `SLURM_GATEWAY_MODE=sbatch`, an allowed script list, an allowed partition list, and a configured script root. The frontend does not hold private keys; mTLS is a gateway boundary.

The same Go backend now exposes the first Simulation Ops control-plane slice:

- `POST /api/simops/runs` creates a bounded run from an allowed scenario/work script and returns MoQ/WebTransport subscription metadata.
- `GET /api/simops/runs/{run_id}` returns run, worker, spool-command, and Iceberg artifact-planning state.
- `POST /api/simops/runs/{run_id}/stop` records a controlled stop request.
- `POST /internal/simops/runs/{run_id}/ingest` accepts token-gated telemetry batches from Rust bucket workers.

The local deployment model assigns Postgres to control-plane and Iceberg catalog metadata, Redpanda to the hot telemetry log, MinIO to S3-compatible Iceberg storage, and separate `simops-stream-gateway` and `simops-iceberg-writer` service boundaries. The checked-in Go slice exposes memory-backed contract adapters plus health/readiness surfaces; real Postgres, broker, MoQ, Docker-launcher, and Iceberg client modules remain explicit integration seams rather than browser-visible credentials.

```bash
bun run backend:test
bun run certs:local
SLURM_GATEWAY_TLS_CERT_FILE=.local/certs/server.crt \
SLURM_GATEWAY_TLS_KEY_FILE=.local/certs/server.key \
SLURM_GATEWAY_CLIENT_CA_FILE=.local/certs/ca.crt \
bun run backend:run
```

## Version 2 Worktree

```bash
git worktree add ../radiant-prj-v2 -b codex/v2-quality-docs main
```

Version 2 work is isolated on `codex/v2-quality-docs` in the sibling `radiant-prj-v2` worktree.

## Version Checkpoints

```bash
scripts/checkpoint-wip.sh --dry-run --skip-checks --no-push
scripts/fold-branch.sh --dry-run --skip-checks --source-branch codex/v2-quality-docs --target-branch main
scripts/checkpoint-version.sh --version v2.1.0 --dry-run --skip-checks --no-push --unsigned
scripts/cleanup-version-hygiene.sh --version v2.1.0 --dry-run --skip-checks --no-push --unsigned
```

`scripts/checkpoint-wip.sh` creates a recoverable WIP checkpoint commit on the current branch.
`scripts/fold-branch.sh` folds a source branch into the worktree that has the target branch checked out.
`scripts/checkpoint-version.sh` runs release verification, stages controlled source files while excluding `JD.mhtml` and generated/build output, creates the requested version checkpoint tag, and optionally pushes the branch and tag.
`scripts/cleanup-version-hygiene.sh` verifies the target branch, creates or confirms the requested version tag, optionally pushes branch and tag, and can remove a supplied extra worktree or merged branch.
The v2-specific `scripts/fold-v2-to-main.sh`, `scripts/checkpoint-v2.sh`, and `scripts/cleanup-v2-hygiene.sh` wrappers remain available for historical v2 commands.

The existing `scripts/checkpoint-v1.sh` remains available for the historical v1 checkpoint flow.

## Repository Map

- `src/data/readiness-fixtures.json` is the controlled fixture source for public facts, jobs, requirements, compute evidence, controlled process evidence, milestones, and deployment checks.
- `src/domain/readiness.ts` contains deterministic toy calculations, diagnosis rules, evidence hashing, and traceability checks.
- `backend/slurm-gateway/` contains the v3.0 mock-first Slurm gateway handlers and tests.
- `deploy/slurm-gateway.compose.yml` defines the SimOps control-plane, Redpanda, Postgres, MinIO, `simops-stream-gateway`, `simops-iceberg-writer`, and Rust bucket service topology.
- `deploy/postgres-init/001_simops.sql` defines the SimOps control-plane and Iceberg SQL-catalog tables used by the local deployment.
- `docs/schemas/simulation-ops/` and `examples/simulation-ops/` define the Simulation Ops telemetry contract and canonical example run artifacts.
- `docs/requirements/` contains the requirements, verification matrix, change log, and objective evidence index.
- `docs/quality/` contains quality program, document control, configuration management, lifecycle, V&V, corrective action, records, tool, supplier, release readiness, and document-index procedures.
- `docs/design/` contains software design and interface-control records.
- `docs/verification/` contains verification plan, test procedure, and test report template.
- `docs/release/` contains checklist, baseline, approval, review minutes, and version-history records.
- `infra/terraform/` declares dry-run hybrid compute environment intent only.
- `infra/ansible/` contains local-safe Linux/HPC baseline templates.
- `.github/workflows/ci.yml` mirrors the verification path.

## Boundaries

The transport, thermal, fleet, and infrastructure records are synthetic. They are designed to demonstrate engineering judgment, reproducibility, traceability, and HPC operations fluency without representing any real Kaleidos analysis or Radiant infrastructure.
