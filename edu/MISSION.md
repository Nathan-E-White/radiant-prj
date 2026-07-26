# Mission: Efficient, trustworthy GitHub Actions

## Why
Reduce wasted GitHub Actions time and downloads in Radiant while keeping the checks that protect main meaningful. The practical outcome is being able to change CI confidently rather than treating it as an expensive black box.

## Success looks like
- Diagnose why a workflow ran and distinguish `pull_request` from `push` runs.
- Safely restrict duplicate runs and cancel obsolete work.
- Split heavyweight checks by changed paths while preserving the required `verify` merge gate.

## Constraints
- Preserve the active `main-hygiene` ruleset and its required `verify` status check.
- Keep security scanning and post-merge validation; optimise developer feedback first.
- Make small, reviewable workflow changes with visible evidence.

## Out of scope
- Rebuilding application tests or changing GitHub billing plans.
