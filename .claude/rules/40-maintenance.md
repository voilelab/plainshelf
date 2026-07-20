# 40 — Rule maintenance

This policy covers `CLAUDE.md`, `.claude/rules/*.md`, and project-local Claude
skills. The goal is a small active rule set grounded in current repository facts.

## File responsibilities

- `CLAUDE.md`: product invariants, repository map, canonical commands, routing
- `10-delegation.md`: optional collaboration mechanics
- `20-judgment.md`: scope, escalation, and completion standards
- `30-prompt-templates.md`: reusable task briefs
- `50-lessons.md`: concise, verified, project-specific pitfalls
- `00-diagnosis.md`, `90-letter.md`: historical snapshots; do not edit
- `.claude/skills/`: repeatable project workflows

Put a rule in exactly one active file and link to it elsewhere. Do not copy the
same command, threshold, or explanation across multiple files.

## What belongs in lessons

Add a lesson only when all are true:

1. the behavior was reproduced or traced to code;
2. it is likely to recur;
3. the fix is not obvious from the failing command;
4. a short instruction can prevent meaningful future effort.

Use this format:

```text
- **Topic:** symptom or trigger → verified cause → next action. (`relevant/path`)
```

Keep details in code comments, tests, issue/PR history, or durable technical docs.
Lessons are an index of traps, not a chronological incident log.

## Change procedure

1. Verify commands and paths against the current checkout.
2. Update every direct reference when a rule or document moves.
3. Read back the changed files and search for stale terminology.
4. Run a Markdown/link or docs build check when available.
5. Review `git diff` for accidental policy changes.

User requests to organize or revise the rules authorize coherent cleanup within
that scope. Otherwise, ask before deleting a product constraint, weakening a
safety boundary, adding automatic hooks, or introducing a new skill/agent.

## Size budget

- Keep `CLAUDE.md` under roughly 100 lines.
- Keep each active rule focused enough to scan in under two minutes.
- When lessons become repetitive or exceed about 40 entries, merge by root cause
  and move durable explanations into the closest code or documentation page.
- Prefer deleting obsolete active guidance; Git history is the archive.
