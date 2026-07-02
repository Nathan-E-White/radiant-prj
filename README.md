# Kaleidos Compute Readiness Console

Interview-facing engineering console for a Radiant application packet. The app combines source-linked public facts about Kaleidos with synthetic compute jobs, HPC failure triage, DevOps deployment artifacts, and a controlled requirements/evidence framework.

This is public-safe demo material. It is not reactor design, safety analysis, licensing evidence, or proprietary Radiant work.

## What It Shows

- **Kaleidos Brief:** public-source product facts, a stylized cutaway, readiness milestones, and explicit claim boundaries.
- **Compute Workbench:** deterministic toy transport, thermal, fleet, and infrastructure jobs with scheduler states, logs, artifacts, and diagnosis.
- **Evidence Matrix:** requirements, verification methods, evidence packs, artifact hashes, deployment checks, and controlled change notes.
- **DevOps Layer:** Docker Compose, dry-run Terraform, Ansible baseline templates, and CI checks for the synthetic hybrid compute environment.
- **Version 2 Quality Package:** controlled quality, design, verification, release, and records documentation suitable for serious engineering review.

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
```

`bun run ci` runs the full local verification chain.

## Version 2 Worktree

```bash
git worktree add ../radiant-prj-v2 -b codex/v2-quality-docs main
```

Version 2 work is isolated on `codex/v2-quality-docs` in the sibling `radiant-prj-v2` worktree.

## Version Checkpoints

```bash
scripts/checkpoint-wip.sh --dry-run --skip-checks --no-push
scripts/fold-v2-to-main.sh --dry-run --skip-checks
scripts/checkpoint-v2.sh --dry-run --skip-checks --no-push --unsigned
scripts/cleanup-v2-hygiene.sh --dry-run --skip-checks --unsigned
```

`scripts/checkpoint-wip.sh` creates a recoverable WIP checkpoint commit on the current branch.
`scripts/fold-v2-to-main.sh` folds the v2 branch into the worktree that has `main` checked out.
`scripts/checkpoint-v2.sh` runs release verification, stages controlled source files while excluding `JD.mhtml` and generated/build output, creates a `v2.0.0` checkpoint tag, and optionally pushes the branch and tag.
`scripts/cleanup-v2-hygiene.sh` verifies `main`, creates or confirms the `v2.0.0` tag, pushes branch and tag, removes the extra v2 worktree, and deletes the merged v2 branch.

The existing `scripts/checkpoint-v1.sh` remains available for the historical v1 checkpoint flow.

## Repository Map

- `src/data/readiness-fixtures.json` is the controlled fixture source for public facts, jobs, requirements, evidence, milestones, and deployment checks.
- `src/domain/readiness.ts` contains deterministic toy calculations, diagnosis rules, evidence hashing, and traceability checks.
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
