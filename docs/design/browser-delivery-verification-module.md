# Browser Delivery Verification Module

| Field | Value |
| --- | --- |
| Module | Browser Delivery Verification |
| Lifecycle state | Active capability |
| Owner role | Software, with Quality review |
| Decision record | [ADR-0009](../adr/adr-0009.md) |

## Purpose

Establish that the browser product can be typed, tested, built for production, and delivered within its defined output budgets, then exercised through its tracked browser acceptance journeys.

## Owned Contract

`bun run browser:verify` owns the combined browser-delivery claim: TypeScript checking, the complete frontend suite, a production build, and entry, lazy, raster, and total output budgets. `bun run test:e2e` owns the browser-acceptance claim: the full tracked Playwright suite starts and stops the local application server and browser through `playwright.config.ts`, and covers Fleet Board and Simulator Workbench journeys. Neither command owns backend, container-image, or infrastructure verification.

## Highest Verification Seam And Evidence

The delivery seam is `bun run browser:verify`; its command result and production-build budget report are the evidence source. The acceptance seam is `bun run test:e2e`; Playwright names a failing spec and scenario in its console output, while CI retains `playwright-report/` and `test-results/` as diagnostics. The claims are retained by `browser.delivery` and `browser.acceptance` in `config/repository-verification.json`; CI runs the complete acceptance suite in its `browser acceptance` job.

## Controlled Record Links

- [Quality Plan](../quality/quality-plan.md)
- [Configuration Management Procedure](../quality/configuration-management.md)
- [Requirements-to-Verification Matrix](../requirements/verification-matrix.md)
- [Verification Plan](../verification/verification-plan.md)
- [Corrective-Action Procedure](../quality/corrective-action.md)

## Lifecycle Note

This is an active capability. It is separate from Docker delivery verification because a browser bundle can be valid while a container delivery contract is not, and vice versa.
