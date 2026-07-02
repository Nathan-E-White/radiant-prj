# Release Readiness Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-010 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Quality |
| Parent Plan | QP-001 |

## Purpose

This procedure defines the controls for preparing, reviewing, checkpointing, and tagging a release baseline.

## Entry Criteria

- Work is isolated on the intended branch or worktree.
- Requirements, verification matrix, objective evidence index, and change log are current.
- Controlled documentation includes owner, revision, status, and baseline metadata.
- No blocking corrective actions remain open.

## Verification Criteria

The release candidate shall pass local CI, build, quality documentation checks, fixture validation, evidence generation, and infrastructure checks.

## Release Actions

1. Complete the release checklist.
2. Run `scripts/checkpoint-v2.sh --dry-run --skip-checks --no-push`.
3. Run the full verification chain.
4. Run `scripts/checkpoint-v2.sh` with the selected push/signing options.
5. Record tag, commit, verification summary, and unresolved limitations.

## Exit Criteria

The release baseline is complete when the v2 tag exists, release records identify the tagged commit, and required verification evidence is available.

