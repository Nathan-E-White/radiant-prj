# Verification and Validation Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-005 |
| Revision | 2.0 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Parent Plan | QP-001 |

## Purpose

This procedure defines how the project plans, performs, records, and reviews verification and validation activities.

## Verification Methods

| Method | Use |
| --- | --- |
| Test | Deterministic unit or integration checks with automated pass/fail criteria |
| Analysis | Documented evaluation of computed or derived results |
| Inspection | Review of source, documentation, fixtures, or UI claim boundaries |
| Demonstration | Execution of user-facing workflow against controlled inputs |
| Configuration audit | Check that files, scripts, infrastructure, and records match the baseline |

## Required Local Verification

The release candidate shall run the same verification path used by CI or a documented local equivalent:

```bash
bun run ci
bun run build
```

## Evidence Requirements

Verification records shall identify requirement IDs, method, command or procedure, result, date or baseline, and objective evidence location. Generated indexes are acceptable as derived evidence when they can be recreated from controlled fixtures and scripts.

## Review of Failures

Failed verification shall be recorded as an issue or corrective action when it affects release readiness. A fix shall update the affected requirement, verification matrix, test, fixture, or procedure before closure.

