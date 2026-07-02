# System Requirements

| Field | Value |
| --- | --- |
| Document ID | REQ-001 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Systems |
| Baseline | v2 candidate |

This document defines controlled system-level objectives for the Kaleidos Compute Readiness Console. The project is a public-safe synthetic demonstration and does not contain proprietary reactor design data, safety analysis, licensing evidence, or production infrastructure records.

## Design Inputs

| Input ID | Source | Use |
| --- | --- | --- |
| DI-001 | Public NRC Kaleidos pre-application activity page | Bounds public architecture claims |
| DI-002 | Public Radiant product and program pages | Bounds mission, test-readiness, and deployment-context claims |
| DI-003 | Interview-facing compute-readiness objective | Drives synthetic workbench and evidence views |
| DI-004 | Quality documentation objective | Drives v2 controlled documentation and release process |

| ID | Requirement | Rationale | Verification | Status |
| --- | --- | --- | --- | --- |
| SR-001 | The console shall distinguish public facts from synthetic analysis outputs at every user-facing claim boundary. | Publicly sourced product information must not be confused with real analysis. | Inspection | Verified |
| SR-002 | The workbench shall provide a deterministic transport-style toy calculation with reproducible scalar flux, leakage, and balance metrics. | The demo should reflect computational transport fluency while staying safely synthetic. | Test | Verified |
| SR-003 | The readiness bundle shall include passive thermal margin and load-following toy checks linked to test-readiness evidence. | Reactor-adjacent readiness needs a thermal thread, even in toy form. | Analysis | Verified |
| SR-004 | The project shall maintain a requirements-to-evidence matrix with artifact hashes and controlled status. | Traceability and objective evidence are core to high-consequence engineering software practice. | Configuration audit | Verified |
| SR-005 | The project shall maintain controlled quality, design, verification, and release documentation for the v2 baseline. | Reviewers should be able to inspect the engineering-control story without relying on oral explanation. | Configuration audit | Draft |
| SR-006 | The release process shall provide WIP, fold-back, and version checkpoint scripts with dry-run capability. | Baseline transitions should be reproducible and recoverable. | Configuration audit | Draft |

## Public-Claim Boundary

The application may present only source-linked public facts about Kaleidos and Radiant. All calculations, logs, job states, deployment checks, and evidence packs are synthetic.

## Traceability Notes

SR-001 through SR-004 are represented in the controlled fixture set. SR-005 and SR-006 control v2 documentation and release-process additions outside the runtime fixture set.

## External Sources

- NRC Kaleidos pre-application activity page: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>
- Radiant homepage: <https://www.radiantnuclear.com/>
- Radiant DOME/PDSA announcement: <https://www.radiantnuclear.com/blog/doe-pdsa-approval/>
- Radiant R-50 factory site license announcement: <https://www.radiantnuclear.com/blog/factory-site-license/>
- Radiant Buckley Space Force announcement: <https://www.radiantnuclear.com/blog/buckley-space-force/>
