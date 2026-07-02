# Software Lifecycle Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-004 |
| Revision | 2.0 |
| Status | Draft for v2.1 review |
| Owner | Software |
| Parent Plan | QP-001 |

## Purpose

This procedure defines the lifecycle controls used for software changes in the v2 baseline.

## Lifecycle Stages

| Stage | Required Output | Acceptance Signal |
| --- | --- | --- |
| Planning | Change summary and affected records | Scope is reviewable |
| Design | Updated design or interface document when behavior changes | Traceability to requirements |
| Implementation | Source, fixtures, docs, and scripts | Local build and tests pass |
| Verification | Test output and evidence generation | Verification matrix satisfied |
| Review | Review comments or approval record | Open findings resolved or dispositioned |
| Baseline | WIP checkpoint or version tag | Checkpoint script completes |

## Software Classification

The application is a public-safe synthetic engineering demonstration. Software changes are still controlled because the demonstration relies on disciplined traceability, deterministic checks, and credible engineering records.

## Design Inputs

Design inputs include public-source fact boundaries, interview demonstration objectives, deterministic toy computation needs, infrastructure-readiness scenarios, and the requirements baseline.

## Design Outputs

Design outputs include React UI behavior, domain logic, controlled fixtures, infrastructure artifacts, scripts, tests, and documentation.

## Acceptance Criteria

A software change is acceptable when affected tests pass, traceability remains complete, public and synthetic content are separated, documentation is current, and the release checklist has no unresolved blocking items.

