# 30 — Task brief templates

Use these as short briefs for agents or as checklists for local work. Replace all
placeholders and remove irrelevant lines.

## Explore

```text
Outcome: Find {implementation/behavior} so that {decision it enables}.
Scope: Search {paths}; do not edit files.
Questions: {specific questions}.
Evidence: Return conclusions with file:line references, plus uncertainties.
```

## Implement

```text
Outcome: Change {behavior} because {user-visible or technical reason}.
Scope: Edit {paths}; do not change {contracts/unrelated areas}.
Acceptance:
- {observable behavior}
- {test or regression case}
Checks: Run {commands}.
Evidence: Summarize changed paths, checks, and remaining risks.
```

## Refactor

```text
Outcome: Improve {structure} while preserving {behavioral contract}.
Scope: {paths and boundaries}.
Invariants: {API, data format, UI, performance, or compatibility constraints}.
Acceptance: Existing tests pass; add tests only for uncovered behavior.
Evidence: Explain the structural change, diff scope, and commands run.
```

## Research

```text
Decision: Determine {question} for {why it matters}.
Sources: Prefer repository code/docs, then primary official sources.
Constraints: {versions, platform, date, or product boundary}.
Evidence: Separate verified facts from inference and cite source locations/links.
```

## Review

```text
Review target: {diff, paths, or artifact}.
Contract: {requirements and invariants}.
Focus: Correctness, regressions, security/data risk, missing tests, and docs drift.
Checks: Run {commands} when practical.
Evidence: List actionable findings by severity with file:line references; state
explicitly when no findings remain.
```

## Brief quality check

Before dispatching, confirm the brief includes a concrete outcome, bounded scope,
observable acceptance criteria, and an evidence format. Avoid prescribing an
implementation unless the design is already decided.
