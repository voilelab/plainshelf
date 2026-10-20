# Agent guidance

PlainShelf keeps project policy and workflows in tool-neutral Markdown. Start
with the root `AGENTS.md`, then follow its required guidance and task routing.
These files are specific to PlainShelf; copying them to another project requires
adapting product constraints, commands, branches, and documentation paths.

This directory is excluded from the MkDocs site, including this index and every
workflow. Open it in the repository; it is not part of the published user docs.

## Loading map

| Entry point | Shared source | When read |
|---|---|---|
| Root `AGENTS.md` | Product constraints, commands, and routing | Every task |
| Root `CLAUDE.md` | Imports `AGENTS.md` | Claude Code startup |
| `.claude/rules/20-judgment.md` | [Judgment](judgment.md) | Claude Code startup; other agents open explicitly |
| `.claude/rules/40-maintenance.md` | [Maintenance](maintenance.md) | Claude Code startup; other agents open explicitly |
| `.claude/rules/50-lessons.md` | [Lessons](lessons.md) | Claude Code startup; other agents read relevant sections |
| `.claude/skills/work-ticket/SKILL.md` | [Ticket workflow](workflows/work-ticket.md) | A tracked-ticket task |
| `.claude/skills/update-docs/SKILL.md` | [Documentation workflow](workflows/update-docs.md) | Documentation synchronization |
| `.claude/skills/unslop/SKILL.md` | [Unslop workflow](workflows/unslop.md) | Prose cleanup; manual `/unslop` only |

Claude rule adapters use imports relative to the adapter file. Skill adapters
retain their discovery metadata and explicitly require opening the workflow;
they do not rely on recursive skill imports. Other tools must read `AGENTS.md`
and its required files, either through native discovery or an explicit prompt.
A Markdown link alone does not guarantee automatic loading in every tool.

## Review a guidance change

Check both the content and the loading path. Product boundaries, test gates,
escalation, PR review, and merge/issue-transition restrictions must remain in
force when instructions move. Read through a fresh Claude Code session and a
session using `AGENTS.md`, including every skill workflow. A docs build verifies
links and rendering; it does not establish that an agent loaded or followed the
instructions.

Keep the lessons together for now so Claude retains its existing startup
coverage. Environment observations in lessons describe the incident recorded;
verify current tool versions, network access, and available capabilities before
applying an environment-specific workaround.
