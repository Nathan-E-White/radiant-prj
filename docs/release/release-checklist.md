# Release Checklist

| Field | Value |
| --- | --- |
| Document ID | REL-001 |
| Revision | 2.0 |
| Status | Template |
| Owner | Quality |
| Target Version | v2.0.0 |

## Checklist

| Item | Status | Evidence |
| --- | --- | --- |
| Work isolated on `codex/v2-quality-docs` | TBD | `git status --short --branch` |
| Requirements documents updated | TBD | `docs/requirements/` |
| Design documents updated | TBD | `docs/design/` |
| Quality procedures updated | TBD | `docs/quality/` |
| Verification documents updated | TBD | `docs/verification/` |
| Release records prepared | TBD | `docs/release/` |
| Fixture validation passed | TBD | `bun run validate:fixtures` |
| Unit tests passed | TBD | `bun run test` |
| Quality documentation check passed | TBD | `bun run quality:check` |
| Build passed | TBD | `bun run build` |
| Full local CI passed | TBD | `bun run ci` |
| WIP checkpoint script dry-run passed | TBD | `scripts/checkpoint-wip.sh --dry-run --skip-checks --no-push` |
| Fold script dry-run passed | TBD | `scripts/fold-v2-to-main.sh --dry-run` |
| Version checkpoint script dry-run passed | TBD | `scripts/checkpoint-v2.sh --dry-run --skip-checks --no-push` |
| Open findings dispositioned | TBD | Corrective-action log or release notes |

## Release Decision

| Decision | Name | Date | Notes |
| --- | --- | --- | --- |
| Proceed / Hold | TBD | TBD | TBD |

