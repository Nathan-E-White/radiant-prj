# GitHub Actions efficiency resources

## Knowledge

- [GitHub Docs: Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
  Authoritative trigger, path-filter, concurrency, and skipped-required-check behaviour. Use before changing `on:`.
- [GitHub Docs: Workflow configuration options for code scanning](https://docs.github.com/en/code-security/reference/code-scanning/workflow-configuration-options)
  CodeQL advanced-setup options, including scoped scanning paths. Use when replacing default CodeQL setup.
- [GitHub Docs: Two CodeQL workflows](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/two-codeql-workflows)
  Explains the interaction between CodeQL default and advanced setup. Use before adding a repository CodeQL workflow.

## Wisdom (Communities)

- [GitHub Community: Actions](https://github.com/orgs/community/discussions/categories/actions)
  Useful for platform-specific behaviour or edge cases after consulting the documentation.

## Gaps

- Establish actual per-job duration and cache-hit data from several Radiant runs before deciding whether any test should move to a schedule.
