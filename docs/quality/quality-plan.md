# Quality Plan

| Field | Value |
| --- | --- |
| Document ID | QP-001 |
| Revision | 2.1 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Effective Baseline | v2.1 candidate |

## Purpose

This plan defines the controlled practices used for the Kaleidos Compute Readiness Console v2.1 baseline. It establishes the documentation, configuration, verification, traceability, and records controls expected for a professional engineering demonstration.

## Scope

The controlled scope includes source code, fixtures, infrastructure artifacts, scripts, requirements, verification records, and generated objective evidence indexes. The scope excludes proprietary reactor design data, licensing submittals, safety analysis, and production infrastructure.

## Quality Objectives

- Maintain traceability from design inputs to requirements, implementation artifacts, verification activities, and objective evidence.
- Keep public-source facts, synthetic calculations, and controlled records visibly separated.
- Make every release checkpoint reproducible from source-controlled procedures.
- Preserve reviewable change history with documented verification and approval evidence.

## Responsibilities

| Role | Responsibility |
| --- | --- |
| Quality | Owns document control, release readiness, records, and issue disposition. |
| Systems | Owns public claim boundaries, design inputs, and system requirements. |
| Software | Owns implementation, unit tests, build behavior, and UI traceability. |
| Infrastructure | Owns Docker, Terraform, Ansible, scheduler, and worker baseline artifacts. |
| Reviewer | Confirms records are complete and acceptance criteria are satisfied before release. |

## Controlled Processes

The v2.1 baseline uses the process documents in this directory as the controlling procedures for document control, configuration management, software lifecycle, V&V, corrective action, records, tool control, supplier control, and release readiness.

## Required Release Evidence

- Requirements baseline and verification matrix.
- Objective evidence index generated from controlled fixtures.
- CI/build output or local equivalent.
- Release checklist and approval record.
- Change log with affected records and verification references.
