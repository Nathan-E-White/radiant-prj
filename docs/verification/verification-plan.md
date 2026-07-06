# Verification Plan

| Field | Value |
| --- | --- |
| Document ID | VVP-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Baseline | v3.0 candidate |

## Purpose

This plan defines verification activities for the v3.0 baseline.

## Verification Scope

The scope includes application type checking, frontend/domain unit tests, Go backend gateway tests, fixture validation, generated evidence, infrastructure artifact checks, quality documentation checks, Simulation Ops contract checks, Simulator Workbench contract checks, resident SCADA stand-in tests, SimOps generator tests, Simulator Workbench backend dataflow smoke, and production build.

## Verification Activities

| Activity | Command | Evidence |
| --- | --- | --- |
| Type checking | `bun run typecheck` | Command output |
| Unit tests | `bun run test` | Vitest output |
| Backend gateway tests | `bun run backend:test` | Go test output |
| Fixture validation | `bun run validate:fixtures` | Validation output |
| Evidence generation | `bun run evidence:generate` | Generated index under `generated/` |
| Infrastructure checks | `bun run infra:check` | Static and optional native check output |
| Quality documentation checks | `bun run quality:check` | Documentation check output |
| Simulation Ops contract checks | `bun run simops:contract:check` | Contract validation output |
| Simulator Workbench contract checks | `bun run simulator-workbench:contract:check` | Contract validation output |
| Resident SCADA stand-in tests | `bun run scada:standins:test` | Rust test output |
| SimOps generator tests | `bun run simops:generator:test` | Rust test output |
| Simulator Workbench backend dataflow smoke | `bun run simulator-workbench:dataflow:smoke` | Docker smoke output for Redpanda, Postgres, Iceberg, and read APIs |
| Production build | `bun run build` | Build output |
| Full local CI | `bun run ci` | Combined command output |

## Acceptance Criteria

- Required commands complete successfully.
- Requirements and evidence references remain traceable.
- Generated evidence can be recreated from controlled fixtures.
- Slurm gateway handlers reject missing or unauthorized certificates and validate job requests before spooling.
- Simulation Ops contract examples validate against the documented envelope, payload, manifest, and summary schemas.
- Simulator Workbench backend dataflow proves measured SCADA frames, SimOps telemetry, simulated results, and imputed twin state through Redpanda, Postgres, Iceberg, and read-only APIs.
- Release scripts pass dry-run checks.
- No blocking findings remain open in release records.
