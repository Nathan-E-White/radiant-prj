# Capability Removal Policy Module

| Module | Capability Removal Policy |
| --- | --- |
| Lifecycle | Active |
| Owner role | Software |
| Highest verification seam | `bun run capability:removal:check -- <base>` |
| Evidence source | Local Git diff, Capability Ledger, Repository Verification manifest, and `config/capability-change-evidence.json` |

The policy is the change-review adapter for the Capability Ledger. It compares an explicit Git base with the proposed repository state, reports active capabilities affected by the changed paths, and fails when a mapped artifact or Repository Verification claim disappears without a controlled disposition.

Declarations are intentionally narrow. A `preserve` declaration records a mapped-artifact move while the capability remains active. A `retire` declaration requires the retained Ledger record to be `intentionally-retired`. A `supersede` declaration requires a successor record that remains active and has a current verification claim. Each declaration names `capabilityId`, `action`, and `removed`: `{ "kind": "artifact", "path": "…" }` or `{ "kind": "verification-claim", "id": "…" }`. Source sets are deliberately not file-level removal gates: this preserves internal refactoring freedom while artifacts and claims retain their stronger lifecycle protection.

The command is identical locally and in CI. Workflow YAML supplies the base revision only; policy, lifecycle evidence, and capability mappings remain repository-owned data rather than conventions embedded in the workflow.
