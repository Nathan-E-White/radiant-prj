# Browser Delivery Verification Module

| Field | Value |
| --- | --- |
| Module | Browser Delivery Verification |
| Lifecycle state | Active capability |
| Owner role | Software, with Quality review |
| Decision record | [ADR-0009](../adr/adr-0009.md) |

## Purpose

Establish that the browser product can be typed, tested, built for production, and delivered within its defined output budgets.

## Owned Contract

`bun run browser:verify` is the module boundary. It owns the combined browser-delivery claim: TypeScript checking, the complete frontend suite, a production build, and entry, lazy, raster, and total output budgets. It does not own backend, container-image, or infrastructure verification.

## Highest Verification Seam And Evidence

The highest seam is `bun run browser:verify`; its command result and production-build budget report are the evidence source. The claim is retained by `browser.delivery` in `config/repository-verification.json`.

## Controlled Record Links

- [Quality Plan](../quality/quality-plan.md)
- [Configuration Management Procedure](../quality/configuration-management.md)
- [Requirements-to-Verification Matrix](../requirements/verification-matrix.md)
- [Verification Plan](../verification/verification-plan.md)
- [Corrective-Action Procedure](../quality/corrective-action.md)

## Lifecycle Note

This is an active capability. It is separate from Docker delivery verification because a browser bundle can be valid while a container delivery contract is not, and vice versa.
