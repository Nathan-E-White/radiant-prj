# Records Management Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-007 |
| Revision | 2.0 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Parent Plan | QP-001 |

## Purpose

This procedure defines how quality records are identified, retained, regenerated, and reviewed.

## Record Types

| Record | Location | Retention Method |
| --- | --- | --- |
| Requirements baseline | `docs/requirements/` | Source controlled |
| Verification matrix | `docs/requirements/verification-matrix.md` | Source controlled |
| Evidence index | `docs/requirements/objective-evidence-index.md`, `generated/evidence-index.json` | Source plus reproducible generation |
| Release checklist | `docs/release/` | Source controlled record template and completed release record |
| Test/build output | CI logs or local release notes | Linked from release record |
| Review minutes | `docs/release/review-minutes-template.md` or PR review | Source controlled or external record |

## Generated Records

Generated files under `generated/` are reproducible local records. They are not source-controlled unless a release explicitly promotes a generated snapshot into a controlled release record.

## Retention

Version tags, checkpoint commits, and release records provide the durable retrieval path for a baseline. Records shall not depend on local build artifacts alone.

