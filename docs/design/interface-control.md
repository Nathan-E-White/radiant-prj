# Interface Control Document

| Field | Value |
| --- | --- |
| Document ID | ICD-001 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Software |
| Baseline | v2 candidate |

## Purpose

This document identifies internal and operational interfaces that are controlled for the v2 baseline.

## User Interface

| Interface | Inputs | Outputs | Control |
| --- | --- | --- | --- |
| Kaleidos Brief | Public facts and milestones | Source-linked fact cards and boundaries | Fixture validation |
| Compute Workbench | Synthetic compute jobs | Job status, logs, outputs, diagnosis | Unit tests and fixture validation |
| Evidence Matrix | Requirements, evidence packs, deployment checks | Traceability and evidence views | Verification matrix and generated evidence |

## Fixture Interface

`src/data/readiness-fixtures.json` shall conform to `src/domain/types.ts`. The fixture set includes public facts, milestones, requirements, compute jobs, evidence packs, and deployment checks.

## Script Interface

| Script | Inputs | Outputs |
| --- | --- | --- |
| `scripts/validate-fixtures.mjs` | Controlled fixtures | Pass/fail fixture validation |
| `scripts/generate-evidence.mjs` | Controlled fixtures | `generated/evidence-index.json` |
| `scripts/check-infra.mjs` | Infrastructure files | Pass/fail static and optional native checks |
| `scripts/check-quality-docs.mjs` | Controlled markdown docs | Pass/fail documentation structure check |
| `scripts/checkpoint-wip.sh` | Git worktree state | WIP checkpoint commit and optional push |
| `scripts/fold-v2-to-main.sh` | v2 branch and main branch | No-fast-forward merge into main |
| `scripts/checkpoint-v2.sh` | Release candidate branch | Version checkpoint commit/tag and optional push |

## Infrastructure Interface

Docker, Terraform, and Ansible files describe local-safe infrastructure intent. They are validated by static checks and optional tool-native checks when the relevant tools are installed.

## External Source Interface

External public-source links are controlled through fixture fields and shall use HTTPS URLs.

