# Repository Verification Module

| Field | Value |
| --- | --- |
| Module | Repository Verification |
| Lifecycle state | Active capability |
| Owner role | Software, with Quality review |
| Decision record | [ADR-0008](../adr/adr-0008.md) |

## Purpose

Provide one deterministic, repository-owned inventory of architecture, delivery, contractual-document, configuration, and executable claims.

## Owned Contract

`config/repository-verification.json` defines retained claims and their evidence adapters; `bun run repository:verify` evaluates them. This module owns claim inventory and traceability, not the underlying browser, backend, Docker, or infrastructure implementation.

## Highest Verification Seam And Evidence

The highest seam is `bun run repository:verify`, with its deterministic report as the immediate evidence source. Individual claims identify their own controlled file, command, Compose, OpenTofu, or document evidence.

## Controlled Record Links

- [Quality Plan](../quality/quality-plan.md)
- [Configuration Management Procedure](../quality/configuration-management.md)
- [Requirements-to-Verification Matrix](../requirements/verification-matrix.md)
- [Verification Plan](../verification/verification-plan.md)
- [Corrective-Action Procedure](../quality/corrective-action.md)

## Lifecycle Note

This is an active capability. It indexes active modules and may retain retired or under-reconciliation status only as explicit metadata; an inventory entry is not itself proof that a capability is active or passing.
