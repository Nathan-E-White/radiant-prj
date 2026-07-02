# Test Procedure

| Field | Value |
| --- | --- |
| Document ID | VVP-002 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Quality |
| Baseline | v2 candidate |

## Purpose

This procedure defines the standard local test sequence for the v2 baseline.

## Preconditions

- The worktree is on the intended branch.
- Dependencies are installed.
- Local generated output may be regenerated.
- No unrelated staged changes are present.

## Procedure

```bash
git status --short --branch
bun run typecheck
bun run test
bun run validate:fixtures
bun run evidence:generate
bun run infra:check
bun run quality:check
bun run build
bun run ci
```

## Expected Result

All commands shall exit with status 0. Any skipped optional native infrastructure checks shall be explicitly reported by `scripts/check-infra.mjs`.

## Failure Handling

Record failures in the release checklist or corrective-action log, identify affected requirements or records, implement the fix, and rerun the affected verification activity plus `bun run ci`.

