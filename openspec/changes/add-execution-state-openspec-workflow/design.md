## Decision

OpenSpec остаётся нормативным источником требований и единственным task ledger. Перед Apply принятого change агент явно активирует `$execution-state` и выбирает route с `source=openspec`.

```text
approved OpenSpec change
        ↓
$execution-state route (source=openspec)
        ├─ passthrough → обычный OpenSpec Apply без state
        └─ lite/reset → statectl overlay в .execution-state/
                            ↓
                     one semantic chunk + checks
                            ↓
                 controller accepts evidence and updates tasks.md
```

## Sources of truth

| Information | Source |
| --- | --- |
| Scope and rationale | `proposal.md` |
| Normative behavior | `specs/` |
| Architecture/design decisions | `design.md` |
| Task graph and status | `tasks.md` |
| Active chunk, checkpoint, lease and short evidence | `.execution-state/state.json` |
| Detailed implementation history | code, checks and Git |

The runtime state never copies full OpenSpec text, never creates `tasks/*.json` and never replaces `.ai/context/`.

## Routing and lifecycle

- `passthrough` is valid only for a short cohesive Apply task without a handoff; no state file is created and ordinary OpenSpec rules continue to apply.
- `lite` or `reset` uses the copied `scripts/statectl.py` and only after the change is accepted.
- A worker never marks a checkbox directly. The controller accepts revision-bound evidence through `statectl complete` and then updates the deterministic OpenSpec task status.
- A new requirement or visual behavior found during Apply blocks the current chunk until the normative OpenSpec artefacts are updated and approved.
- State contains no secrets, passwords, access tokens or PII. `.execution-state/` is local operational state and is excluded from Git.

## Alternatives

- **Manual chat checklist:** rejected: it loses revision-bound evidence and is not portable across context handoffs.
- **State for every short task:** rejected: adds overhead; `passthrough` still enforces routing while avoiding unnecessary state.
- **Replacing OpenSpec task list with JSON:** rejected: `tasks.md` remains the readable, canonical task ledger.
