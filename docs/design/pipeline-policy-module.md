# Pipeline Policy Module

| Field | Value |
| --- | --- |
| Lifecycle state | Active capability |
| Owner role | Software, with Quality review |
| Highest verification seam | `bun run pipeline:policy:check` |
| Evidence source | `config/pipeline-policy.json`, Capability Ledger, and GitHub Actions workflow YAML |

## Purpose

Make CI/CD trust rules executable and reviewable. The module declares each protected workflow's Ledger capability, least-privilege permissions, required commands, and either its untrusted pull-request constraint or its trusted publication boundary.

## Owned Contract

`config/pipeline-policy.json` owns the policy. Workflows are adapters that provide triggers, permissions, and steps; they are deliberately not the policy's sole source because a YAML edit otherwise has no independent statement of the contract it weakened. The verifier reports the policy ID, failing field, expected value, and observed value.

The policy covers the read-only CI verification workflow and the distinct `main` Docker publication workflow. It does not publish an image or change package retention; it merely makes the authority boundary visible before a delivery change tries to cross it.

## Controlled Record Links

- [Quality Plan](../quality/quality-plan.md)
- [Configuration Management Procedure](../quality/configuration-management.md)
- [Requirements-to-Verification Matrix](../requirements/verification-matrix.md)
- [Verification Plan](../verification/verification-plan.md)
- [Corrective-Action Procedure](../quality/corrective-action.md)
