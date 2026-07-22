# Docker Packaging And Delivery Verification Module

| Field | Value |
| --- | --- |
| Module | Docker Packaging and Delivery Verification |
| Lifecycle state | Active capability |
| Owner role | Software, with Quality review |
| Decision record | [ADR-0009](../adr/adr-0009.md) |

## Purpose

Keep the repository's Docker packaging and delivery artifacts bounded, reproducible, and inspectable across the defined image roles.

## Owned Contract

The module owns packaging context, image-content, image-size, browser-asset, builder-cache, and structured-budget evidence. It derives an input identity for every declared image role from the filtered inputs, pinned platform-specific base images, build arguments, and target. A matching trusted registry image may be reused; otherwise the role is built through the common Bake definition. Its retained commands are `bun run docker:packaging:contract:check` and `bun run docker:packaging:verify`.

The pull-request verification path may read reusable registry images and caches but does not publish them. The separate trusted `main` publication workflow may publish identity-tagged images and upload the complete evidence directory. This module does not own the retired Randiant Runtime sandbox or claim that its historical experiment is current runtime evidence.

## Highest Verification Seam And Evidence

The highest seam is `bun run docker:packaging:verify`, which exercises the declared image roles through `docker-bake.hcl` and produces structured evidence. `config/docker-packaging-inputs.json`, `config/docker-packaging-budgets.json`, Docker packaging contract tests, the restricted CI verification workflow, and the trusted `main` publication workflow are the evidence sources. The executable repository claim is `docker-packaging.structured-budgets`.

## Controlled Record Links

- [Quality Plan](../quality/quality-plan.md)
- [Configuration Management Procedure](../quality/configuration-management.md)
- [Requirements-to-Verification Matrix](../requirements/verification-matrix.md)
- [Verification Plan](../verification/verification-plan.md)
- [Corrective-Action Procedure](../quality/corrective-action.md)

## Lifecycle Note

This is an active capability. The former Randiant Runtime sandbox is retired historical context under [ADR-0009](../adr/adr-0009.md) and Issue #140; it is not active verification or delivery evidence. Any incomplete successor must be labelled under reconciliation until its records and evidence are complete.
