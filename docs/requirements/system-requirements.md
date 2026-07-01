# System Requirements

This document defines the controlled system-level objectives for the Kaleidos Compute Readiness Console. The project is a public-safe synthetic demonstration and does not contain proprietary reactor design data, qualified analysis, licensing evidence, or safety conclusions.

| ID | Requirement | Rationale | Verification | Status |
| --- | --- | --- | --- | --- |
| SR-001 | The console shall distinguish public facts from synthetic analysis outputs at every user-facing claim boundary. | Publicly sourced product information must not be confused with real analysis. | Inspection | Verified |
| SR-002 | The workbench shall provide a deterministic transport-style toy calculation with reproducible scalar flux, leakage, and balance metrics. | The demo should reflect computational transport fluency while staying safely synthetic. | Test | Verified |
| SR-003 | The readiness bundle shall include passive thermal margin and load-following toy checks linked to test-readiness evidence. | Reactor-adjacent readiness needs a thermal thread, even in toy form. | Analysis | Verified |
| SR-004 | The project shall maintain a requirements-to-evidence matrix with artifact hashes and controlled status. | Traceability and objective evidence are core to high-consequence engineering software practice. | Configuration audit | Verified |

## Public-Claim Boundary

The application may present only source-linked public facts about Kaleidos and Radiant. All calculations, logs, job states, deployment checks, and evidence packs are synthetic.

## External Sources

- NRC Kaleidos pre-application activity page: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>
- Radiant homepage: <https://www.radiantnuclear.com/>
- Radiant DOME/PDSA announcement: <https://www.radiantnuclear.com/blog/doe-pdsa-approval/>
- Radiant R-50 factory site license announcement: <https://www.radiantnuclear.com/blog/factory-site-license/>
- Radiant Buckley Space Force announcement: <https://www.radiantnuclear.com/blog/buckley-space-force/>
