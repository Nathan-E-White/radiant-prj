# Kaleidos Compute Readiness Console

Interview-facing engineering console for a Radiant application packet. The app combines source-linked public facts about Kaleidos with synthetic compute jobs, HPC failure triage, DevOps deployment artifacts, and a controlled requirements/evidence framework.

This is public-safe demo material. It is not reactor design, safety analysis, licensing evidence, or proprietary Radiant work.

## What It Shows

- **Welcome:** public-source product facts, a stylized cutaway, readiness milestones, and explicit claim boundaries.
- **Status Workbench:** fleet status, measured state, twin viewport, imputed state, simulated result state, lineage, scientific compute queue, SimOps orchestration, and the four-panel HPC status bay.
- **Evidence:** requirements, verification methods, evidence packs, artifact hashes, deployment checks, and controlled change notes.
- **DevOps Layer:** Docker Compose, dry-run Terraform, Ansible baseline templates, and CI checks for the synthetic hybrid compute environment.
- **Version 3.0 Backend Gateway:** mock-first Go Slurm gateway handlers with mTLS identity checks, request validation, job status lookup, and Prometheus-format metrics.
- **Simulation Ops Backend Slice:** Go run control endpoints, API polling events, Postgres-backed run state, Redpanda telemetry publication, Docker-launched Rust worker containers, WebTransport live-track delivery, and Iceberg-Go artifact commits.
- **Simulation Ops Contract:** telemetry schemas, NDJSON examples, WebTransport live-track names, and scenario randomization blueprint for local worker swarms.
- **Status Workbench Backend Dataflow:** resident measured SCADA stand-ins, separate simulated result ingest, twin imputed-state projection, Postgres read models, Iceberg tables, and read-only Workbench APIs backed by the existing Simulator Workbench contracts.
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
bun run simulator-workbench:contract:check
bun run scada:standins:test
bun run simops:generator:test
bun run simops:smoke:local
bun run simulator-workbench:dataflow:smoke
```

`bun run ci` runs the full local verification chain.
`bun run simops:smoke:local` and `bun run simulator-workbench:dataflow:smoke` are Docker-dependent and intentionally stay outside default CI.

## Dev Hygiene Size Audit

The size audit helper reports local storage footprint without deleting files, pruning Docker objects, removing worktrees, or rewriting project state:

```bash
bun run hygiene:size
```

The report separates repo-local storage, registered Git worktrees, external toolchain caches, and Docker/OrbStack storage. Optional paths or tools that are unavailable are reported as skipped instead of failing the command.

Use `bun run hygiene:size:check` to exercise the helper against fake Git, Docker, and Go binaries; it does not change real repo, cache, or Docker state.

## Docker Storage Pruning

The Docker/OrbStack pruning helper defaults to dry-run and prints the selected prune commands without touching Docker state:

```bash
bun run docker:prune:plan
scripts/docker-prune-hygiene.sh --images --build-cache --containers
```

Actual pruning requires `--execute` and explicit categories. `--all` includes volumes in the plan, but volume pruning also requires `--confirm-volumes` during execution because local volumes may hold runtime data:

```bash
scripts/docker-prune-hygiene.sh --images --build-cache --containers --execute
scripts/docker-prune-hygiene.sh --volumes --confirm-volumes --execute
```

Use `bun run docker:prune:check` to exercise the helper against a fake Docker binary; it does not prune real images, build cache, containers, or volumes.

## Version 3.0 Backend Gateway

The v3.0 backend lives under `backend/slurm-gateway`. It exposes `GET /healthz`, `GET /readyz`, `GET /metrics`, `POST /api/jobs/submit`, and `GET /api/jobs/{job_id}`.

Default mode is `mock`, which returns deterministic synthetic Slurm job IDs and keeps the demo public-safe. Real `sbatch` submission is opt-in through `SLURM_GATEWAY_MODE=sbatch`, an allowed script list, an allowed partition list, and a configured script root. The frontend does not hold private keys; mTLS is a gateway boundary.

The same Go backend now exposes the first Simulation Ops control-plane slice:

- `POST /api/simops/runs` creates a bounded run from an allowed scenario/work script and returns WebTransport subscription metadata.
- `GET /api/simops/runs/{run_id}` returns run, worker, spool-command, and Iceberg artifact-planning state.
- `GET /api/simops/runs/{run_id}/events` returns persisted lifecycle, telemetry, and artifact-ready events for recovery and inspection; it is not the live telemetry read path.
- `POST /api/simops/runs/{run_id}/stop` records a controlled stop request.
- `POST /internal/simops/runs/{run_id}/ingest` accepts token-gated operational telemetry batches from Rust bucket workers.
- `POST /internal/simops/runs/{run_id}/results` accepts token-gated synthetic simulated result batches from Rust bucket workers.
- `POST /internal/scada/sources` registers public-safe resident measured source declarations.
- `POST /internal/scada/telemetry` accepts token-gated measured SCADA stand-in frames.
- `GET /api/simulator-workbench/state`, `/measured`, `/twin`, and `/lineage/{value_id}` expose read-only backend dataflow projections for the follow-up frontend-control slice.

The local deployment model assigns Timescale/Postgres to control-plane state, telemetry projection, Workbench read models, and Iceberg SQL catalog metadata. Redpanda carries the hot durable telemetry, measured SCADA, simulated result, and twin-state streams. MinIO stores S3-compatible Iceberg data. `simops-moq-gateway` delivers Redpanda-backed WebTransport tracks, `simops-timescale-writer` and `simops-iceberg-writer` persist operational telemetry, `workbench-projection-writer` persists measured/result/twin projections, `twin-projector` is the only process that emits imputed twin state, and `workbench-iceberg-writer` appends Workbench lake tables. Docker Compose starts the always-on platform. The Go gateway launches run-scoped worker containers on demand, passes them the run id and ingest tokens, validates incoming frames, publishes them to Redpanda, and updates lightweight run/worker counters. Workers never receive broker, database, MinIO, or Iceberg credentials. `bun run simops:smoke:local` verifies Timescale rows, readable Iceberg/Parquet data, and a WebTransport subscriber probe. `bun run simulator-workbench:dataflow:smoke` verifies one measured unit, one operational telemetry unit, one simulated result unit, and one imputed twin value through Redpanda, Postgres, Iceberg, and the read-only Workbench APIs.

For browser-local development, the compose gateway sets `SLURM_GATEWAY_REQUIRE_CLIENT_CERT=false`; mTLS remains the non-browser gateway boundary. The Compose-local smoke path launches worker containers on the `radiant-simops-local` network with `SIMOPS_WORKER_INGEST_BASE_URL=http://slurm-gateway:8080` and a two-frame override to keep the proof bounded. For host-run worker orchestration, use `SIMOPS_WORKER_RUNTIME=docker` and `SIMOPS_WORKER_INGEST_BASE_URL=http://host.docker.internal:8080` or the matching published gateway port.

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
- `deploy/slurm-gateway.compose.yml` defines the SimOps control-plane, Redpanda, Timescale/Postgres, MinIO, `simops-moq-gateway`, `simops-timescale-writer`, `simops-iceberg-writer`, Workbench writers, `twin-projector`, `scada-standins`, and smoke/demo Rust bucket service topology.
- `deploy/postgres-init/001_simops.sql` defines the SimOps control-plane, Timescale telemetry hypertable, Workbench projection tables, twin lineage tables, consumer offsets, and Iceberg SQL-catalog tables used by the local deployment.
- `docs/schemas/simulation-ops/` and `examples/simulation-ops/` define the Simulation Ops telemetry contract and canonical example run artifacts.
- `docs/schemas/scada/`, `docs/schemas/digital-twin/`, `docs/schemas/simulator-workbench/`, `examples/scada/`, `examples/digital-twin/`, and `examples/simulator-workbench/` define the Status Workbench contracts and examples; the path names remain `simulator-workbench` for controlled backend/API continuity.
- `docs/design/simulator-workbench-stub-ledger.md` tracks scaffold seams and acceptance criteria; `docs/design/simulator-workbench-backend-dataflow-slice.md` controls the backend dataflow proof; `docs/design/simulator-workbench-visual-draft.md` stores the first concept image notes; `docs/adr/adr-0006.md` records the user-facing Status Workbench IA decision.
- `workers/scada-standins/` contains the resident measured-source service for Status Workbench backend dataflow work.
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

### Guarded hygiene cleanup

`bun run hygiene:clean` prints a dry-run plan for generated outputs, Rust worker targets, dependency installs in registered worktrees, and named Radiant Go caches. It never deletes Git worktrees, Docker objects, volumes, or arbitrary paths. Select categories explicitly and add `--execute` only when removal is intended:

```sh
bun run hygiene:clean
bun run hygiene:clean --generated --rust-targets --execute
```

The cleanup command prints each selected path before removal and refuses `--execute` without explicit category flags.
