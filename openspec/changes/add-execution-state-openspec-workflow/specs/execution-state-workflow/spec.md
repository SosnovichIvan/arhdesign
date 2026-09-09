## ADDED Requirements

### Requirement: Execution-state routing for OpenSpec Apply

Every Apply task from an accepted OpenSpec change MUST explicitly invoke `$execution-state` and route with `source=openspec` before implementation begins. A `passthrough` decision MAY proceed without creating a state file only for a short cohesive task.

#### Scenario: Short accepted task

- **WHEN** an accepted OpenSpec task is routed as `passthrough`
- **THEN** the agent follows ordinary OpenSpec Apply rules without creating `.execution-state/state.json`, and `tasks.md` remains the only task ledger.

#### Scenario: Long accepted task

- **WHEN** an accepted OpenSpec task is routed as `lite` or `reset`
- **THEN** `execution-state` tracks the active semantic chunk, evidence and handoff while OpenSpec retains requirements and task status.

### Requirement: Runtime state is non-normative and safe

Runtime state MUST remain separate from OpenSpec and project context, MUST be excluded from Git, and MUST contain no secrets, authentication tokens, passwords or PII.

#### Scenario: Completed stateful chunk

- **WHEN** a stateful worker finishes a chunk
- **THEN** only the controller accepts revision-bound evidence and updates the OpenSpec checkbox after declared regression checks pass.
