# Verification Plan

| Field | Value |
| --- | --- |
| Document ID | VVP-001 |
| Revision | 2.0 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Baseline | v2.1 candidate |

## Purpose

This plan defines verification activities for the v2 baseline.

## Verification Scope

The scope includes application type checking, unit tests, fixture validation, generated evidence, infrastructure artifact checks, quality documentation checks, and production build.

## Verification Activities

| Activity | Command | Evidence |
| --- | --- | --- |
| Type checking | `bun run typecheck` | Command output |
| Unit tests | `bun run test` | Vitest output |
| Fixture validation | `bun run validate:fixtures` | Validation output |
| Evidence generation | `bun run evidence:generate` | Generated index under `generated/` |
| Infrastructure checks | `bun run infra:check` | Static and optional native check output |
| Quality documentation checks | `bun run quality:check` | Documentation check output |
| Production build | `bun run build` | Build output |
| Full local CI | `bun run ci` | Combined command output |

## Acceptance Criteria

- Required commands complete successfully.
- Requirements and evidence references remain traceable.
- Generated evidence can be recreated from controlled fixtures.
- Release scripts pass dry-run checks.
- No blocking findings remain open in release records.

