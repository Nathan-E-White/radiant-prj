# Configuration Management Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-003 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Quality |
| Parent Plan | QP-001 |

## Purpose

This procedure defines how the project identifies configuration items, controls changes, establishes baselines, and restores release state.

## Configuration Items

| CI ID | Item | Source of Truth | Baseline Control |
| --- | --- | --- | --- |
| CI-SRC | Application source | `src/` | Git branch, checkpoint commit |
| CI-FIX | Controlled fixtures | `src/data/readiness-fixtures.json` | Fixture validation and traceability tests |
| CI-DOC | Controlled documentation | `docs/` | Document index and release checklist |
| CI-INF | Infrastructure artifacts | `Dockerfile`, `docker-compose.yml`, `infra/` | Static infra checks |
| CI-SCR | Operational scripts | `scripts/` | Dry-run validation and release checkpoint |
| CI-CI | CI workflow | `.github/workflows/ci.yml` | CI and local equivalent |

## Baseline Types

| Baseline | Purpose | Establishing Action |
| --- | --- | --- |
| Working baseline | Active branch or worktree for development | Branch creation or worktree checkout |
| WIP checkpoint | Recoverable intermediate work state | `scripts/checkpoint-wip.sh` |
| Release candidate | Reviewed version-ready state | Completed release checklist |
| Version baseline | Tagged version checkpoint | `scripts/checkpoint-v2.sh` |

## Change Control

Changes shall identify affected CIs, requirements, verification activities, and records. Changes affecting requirements, verification logic, evidence generation, release scripts, or public claim boundaries require review before version checkpoint.

## Status Accounting

The current baseline status is derived from `git status`, branch name, latest checkpoint tag, release checklist, change log, and verification output.

## Recovery

Recovery from an interrupted change shall start from the latest clean branch, WIP checkpoint, or version tag. Generated files may be regenerated and shall not be treated as authoritative source records unless explicitly promoted into release records.

