# Document Control Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-002 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Quality |
| Parent Plan | QP-001 |

## Purpose

This procedure controls creation, review, approval, revision, and retirement of project documentation.

## Controlled Document Classes

| Class | Examples | Control Method |
| --- | --- | --- |
| Plan | Quality plan, V&V plan, release plan | Revision table and approval record |
| Procedure | Configuration, records, corrective action | Revision table and effective baseline |
| Specification | Requirements, design description, interfaces | Traceability to inputs and verification |
| Record | Test report, release checklist, review minutes | Retained as completed evidence |
| Generated record | Evidence index, build output summaries | Regenerated from controlled source |

## Required Metadata

Controlled documents shall include document ID, revision, status, owner, and either a parent plan or baseline reference. Records shall identify date, preparer, reviewer, and source artifacts when applicable.

## Review and Approval

New and revised controlled documents require review by an owner outside the authoring role when practical. Approval may be recorded in release records, review minutes, pull request approval, or signed checkpoint commit metadata.

## Revision Practice

- Minor editorial changes may share the next planned baseline if they do not change requirements, verification, acceptance criteria, or release evidence.
- Technical changes shall update the affected change log, traceability matrix, and verification evidence.
- Retired documents shall remain in history and be identified as superseded in the release baseline.

## Document Index

The release baseline shall include a document index that lists controlled documents, revisions, owners, status, and evidence links.

