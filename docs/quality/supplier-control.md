# Supplier and External Source Control

| Field | Value |
| --- | --- |
| Document ID | QP-009 |
| Revision | 2.0 |
| Status | Draft for v2.1 review |
| Owner | Systems |
| Parent Plan | QP-001 |

## Purpose

This procedure defines controls for external public sources, open-source dependencies, and third-party tools used by the demonstration.

## External Public Sources

Public claims shall include source title, source URL, confidence level, and claim boundary. Public-source claims shall not be expanded beyond what the source supports.

## Open-Source Dependencies

Dependencies are identified in `package.json`. Dependency changes shall be reviewed for purpose, license compatibility, and impact on verification.

## Infrastructure Providers

Terraform and Ansible artifacts define local-safe infrastructure intent only. Provider or deployment changes require review of affected configuration checks and release records.

## Acceptance

External inputs are acceptable when they are traceable, reviewable, and bounded by the project claim controls.

