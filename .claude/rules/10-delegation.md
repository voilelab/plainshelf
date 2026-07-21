# 10 — Collaboration and delegation

Delegation is optional. Use it when the current environment supports agents and
the work can be split into independent, bounded pieces. Do not delegate merely
to avoid reading the code needed for the final decision.

## Good delegation candidates

- Independent searches in different subsystems
- A contained implementation with explicit file boundaries
- Mechanical changes across many files after one example establishes the pattern
- A fresh review of a risky or wide-reaching change
- Long-running tests that can execute independently

Keep work local when it is small, sequential, tightly coupled to an unresolved
design choice, or when coordination costs more than the task.

## Task brief contract

Every delegated task should state:

1. **Outcome:** the concrete result and why it matters.
2. **Scope:** relevant paths, allowed edits, and explicit exclusions.
3. **Acceptance:** observable checks that define success.
4. **Evidence:** the concise result, changed paths, and commands actually run.

Use the templates in `30-prompt-templates.md`. Choose models and reasoning
settings from the capabilities exposed by the current tool; never hard-code a
list from an older session.

## Coordination

- Give agents disjoint write scopes whenever possible.
- Do not let multiple agents edit the same file concurrently.
- Share verified facts and file references, not large raw logs.
- Reuse an agent for a follow-up on the same context; use a fresh context for an
  independent review.
- The coordinating agent owns integration, conflict resolution, and the final
  verification. An agent's success claim is evidence, not proof.

## Failure handling

- First determine whether the failure is code, environment, or a wrong assumption.
- If the same root cause repeats, stop retrying and change the approach.
- Escalate model capability only when the task genuinely needs broader reasoning,
  not when a command or premise is wrong.
- Surface blockers with the exact failed command and the smallest useful error
  excerpt.
