# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Layout

This repo is configured for the multi-context domain-doc layout.

- **`CONTEXT-MAP.md`** at the repo root is the context router when present. It points at one `CONTEXT.md` per context.
- **`docs/adr/`** holds repo-wide architectural decisions.
- **Context-specific ADRs** may live under context-owned docs directories when they exist.

## Before exploring, read these

- Read `CONTEXT-MAP.md` first when it exists, then read each mapped `CONTEXT.md` relevant to the task.
- Read ADRs in `docs/adr/` that touch the area you are about to work in.
- In multi-context areas, also check mapped context directories for context-scoped `docs/adr/` decisions.

If any of these files do not exist, proceed silently. Do not flag their absence or suggest creating them upfront. The `/domain-modeling` skill, reached through `/grill-with-docs` and `/improve-codebase-architecture`, creates them lazily when terms or decisions actually get resolved.

## File structure

Expected multi-context shape:

```text
/
|-- CONTEXT-MAP.md
|-- docs/adr/
|   |-- 0001-system-wide-decision.md
|   `-- 0002-another-system-decision.md
`-- <context>/
    |-- CONTEXT.md
    `-- docs/adr/
        `-- 0001-context-specific-decision.md
```

## Use the glossary's vocabulary

When your output names a domain concept in an issue title, refactor proposal, hypothesis, or test name, use the term as defined in the relevant `CONTEXT.md`. Do not drift to synonyms the glossary explicitly avoids.

If the concept you need is not in the glossary yet, either reconsider the wording or note the gap for `/domain-modeling`.

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly instead of silently overriding it.
