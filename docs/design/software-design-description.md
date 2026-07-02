# Software Design Description

| Field | Value |
| --- | --- |
| Document ID | SDD-001 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Software |
| Baseline | v2 candidate |

## Purpose

This document describes the design of the Kaleidos Compute Readiness Console and its controlled boundaries.

## System Overview

The console is a local React and TypeScript application that presents source-linked public facts, synthetic compute jobs, deterministic toy calculations, infrastructure-readiness artifacts, and objective evidence records. The design emphasizes traceability and reproducibility over runtime complexity.

## Major Components

| Component | Location | Responsibility |
| --- | --- | --- |
| UI console | `src/App.tsx`, `src/styles.css` | Renders brief, workbench, and evidence views from controlled fixtures |
| Domain logic | `src/domain/readiness.ts` | Performs deterministic toy calculations, diagnosis, hashing, and traceability checks |
| Domain types | `src/domain/types.ts` | Defines controlled fixture and result shapes |
| Fixtures | `src/data/readiness-fixtures.json` | Source of public facts, synthetic jobs, requirements, evidence packs, and deployment checks |
| Evidence generation | `scripts/generate-evidence.mjs` | Generates reproducible local evidence index |
| Fixture validation | `scripts/validate-fixtures.mjs` | Enforces fixture integrity and traceability |
| Infrastructure checks | `scripts/check-infra.mjs` | Verifies local-safe infrastructure artifact completeness |

## Design Constraints

- Public facts shall remain source-linked and bounded.
- Synthetic outputs shall remain clearly separated from public facts.
- Evidence indexes shall be reproducible from controlled fixtures.
- Release scripts shall default to excluding generated output, build output, local environment files, and `JD.mhtml`.
- The application shall run locally without a production backend.

## Data Flow

1. Controlled fixtures define facts, requirements, jobs, evidence packs, milestones, and deployment checks.
2. Domain functions compute toy transport, thermal, fleet, diagnosis, evidence, and coverage outputs.
3. The UI renders controlled fixture records and derived outputs.
4. Validation scripts check fixture consistency and infrastructure artifact presence.
5. Evidence generation writes a reproducible derived index under `generated/`.

## Design Outputs

Design outputs are source files, fixture records, test cases, infrastructure artifacts, quality documentation, release scripts, and generated evidence procedures.

