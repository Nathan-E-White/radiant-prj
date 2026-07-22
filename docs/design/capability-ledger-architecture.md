# Capability Ledger Architecture Selection

| Field | Value |
| --- | --- |
| Status | Selected design input for Issue #175 |
| Owner | Software |
| Parent | Issue #173 |

## Problem

Repository Verification can evaluate a claim, but it does not identify the retained capability that owns the claim, the high-level implementation surfaces that make it real, or which active capabilities a changed path affects. A maintainer therefore has to reconstruct those links from history.

## Alternatives Considered

1. Add capability metadata directly to each Repository Verification claim. Rejected: a claim may be shared, while lifecycle, owner, evidence, and source-set mapping are capability concerns.
2. Build a new verification runner. Rejected: it would duplicate the existing command, report, adapter, and CI seam.
3. Keep a declarative Ledger and a small module that delegates named verification to Repository Verification. Selected.

## Selected Module

`scripts/capability-ledger/` is one deep module with three public operations:

- validate the declarative ledger against its lifecycle and Repository Verification invariants;
- verify one named capability by delegating its declared claim to `verifyRepository`; and
- identify active capabilities affected by one or more changed paths.

The Ledger record owns capability identity, lifecycle state, successor relationship, accountable role, decision/source reference, controlled-document references, high-level implementation surfaces, verification claim, operational constraints, and last verification evidence. It links to those records and does not copy their content.

## Surface Mapping

An `artifact` surface names a file only when that file is itself contractual, such as a workflow. A `source-set` names a root and optional extensions when behavior may safely move inside the set. Validation requires a source-set root to exist, not every incidental filename; a behavior-preserving internal move therefore remains valid.

## Lifecycle Rules

Valid states are `active`, `superseded`, `intentionally-retired`, and `under-reconciliation`. A successor, when present, must name a different Ledger record. A superseded record requires a successor. Changed-path lookup returns only active records; reconciliation and removal policy remain later modules under Issues #176 and #177.

## Verification And Failure Model

The named-capability operation validates the Ledger first, resolves the declared claim, and invokes the existing Repository Verification seam with that claim ID. It returns the standard claim report rather than reinterpret command, Compose, OpenTofu, or document evidence. Unknown capability IDs, malformed records, and missing claim references are deterministic, actionable failures.

The [initial reconciliation baseline](../verification/capability-reconciliation-baseline.md) records the first controlled high-risk audit and its present evidence. It is a dated evidence record, not a replacement for the Ledger or for subsequent reconciliation runs.
