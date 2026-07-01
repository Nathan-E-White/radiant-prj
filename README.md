# Kaleidos Compute Readiness Console

Interview-facing engineering console for a Radiant application packet. The app combines source-linked public facts about Kaleidos with synthetic compute jobs, HPC failure triage, DevOps deployment artifacts, and a quiet requirements/evidence framework.

This is public-safe demo material. It is not reactor design, safety analysis, licensing evidence, proprietary Radiant work, or a claim of qualification.

## What It Shows

- **Kaleidos Brief:** public-source product facts, a stylized cutaway, readiness milestones, and explicit claim boundaries.
- **Compute Workbench:** deterministic toy transport, thermal, fleet, and infrastructure jobs with scheduler states, logs, artifacts, and diagnosis.
- **Evidence Matrix:** requirements, verification methods, evidence packs, artifact hashes, deployment checks, and controlled change notes.
- **DevOps Layer:** Docker Compose, dry-run Terraform, Ansible baseline templates, and CI checks for the synthetic hybrid compute environment.

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
```

`bun run ci` runs the full local verification chain.

## Repository Map

- `src/data/readiness-fixtures.json` is the controlled fixture source for public facts, jobs, requirements, evidence, milestones, and deployment checks.
- `src/domain/readiness.ts` contains deterministic toy calculations, diagnosis rules, evidence hashing, and traceability checks.
- `docs/requirements/` contains the requirements, verification matrix, change log, and objective evidence index.
- `infra/terraform/` declares dry-run hybrid compute environment intent only.
- `infra/ansible/` contains local-safe Linux/HPC baseline templates.
- `.github/workflows/ci.yml` mirrors the verification path.

## Boundaries

The transport, thermal, fleet, and infrastructure records are synthetic. They are designed to demonstrate engineering judgment, reproducibility, traceability, and HPC operations fluency without representing any real Kaleidos analysis or Radiant infrastructure.
