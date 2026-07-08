---
description: "Use when project docs (README.md, docs/) are stale relative to recent code changes. Triggers on: update docs, sync docs, docs are out of date, document this feature."
name: "update-docs"
tools: [read, bash, grep, glob, edit]
argument-hint: "Optional: specific doc file, feature name, or commit range to focus on"
---

Bring `README.md` and `docs/**/*.md` back in sync with what the code on the current branch actually does.
This skill is about user-facing documentation (concepts, setup, configuration, usage). It is **not** for `CHANGELOG.md` (use `update-changelog`) and **not** for `CLAUDE.md` / `.claude/rules/*` (those are governed by `.claude/rules/40-maintenance.md`).

## Steps

1. Determine scope:
   - If the user named a specific doc file, feature, or commit range, focus on that.
   - Otherwise, find what changed: `git log $(git describe --tags --abbrev=0)..HEAD --oneline` (or `git log --oneline` if no tags), and `git diff dev...HEAD --stat` for the current branch's full diff against the default branch (`dev`, per CLAUDE.md).
2. For each changed area, read the relevant existing doc(s) first — don't guess file layout:
   - `docs/index.md` — project overview, goals, non-goals, project structure
   - `docs/getting-started.md`, `docs/installation.md` — install/run flows
   - `docs/configuring-local-shelf.md`, `docs/configuring-smb-shelf.md` — shelf configuration
   - `docs/concepts/*.md` — data model, layers, and other durable concepts
   - `docs/development/*.md` — contributor-facing setup (Docker, dev environment)
   - `docs/known-issue.md` — known limitations
   - `README.md` — top-level summary, should stay a thin pointer into `docs/`, not diverge from it
3. Read the actual current code/behavior for anything you plan to document — never document from the commit message alone. For config or API surface, check the real flags/fields/defaults in code.
4. Update only what's now inaccurate or missing:
   - Fix stale instructions, flags, defaults, file paths, or screenshots-worthy behavior descriptions.
   - Add a short section/paragraph for genuinely new user-facing features or setup steps, matching the tone and structure of the surrounding doc (this project uses MkDocs-style Markdown, e.g. `!!! warning` admonitions in `index.md`).
   - Do not invent structure: if a change doesn't fit any existing doc, prefer extending the closest existing section over creating a new file; only create a new file if the user asked for one or the topic is clearly its own concept (mirror `docs/concepts/` granularity).
5. Respect the project's non-goals and pre-alpha framing (`docs/index.md`) — do not soften or contradict them when documenting new work.
6. Leave anything you're not confident about unchanged rather than guessing; note it in your report instead.

## Notes

- Prefer minimal, surgical edits over rewrites — these docs are read by real early users, not just generated.
- If `README.md` and `docs/index.md` would end up saying the same thing in different words, keep `README.md` short and link to `docs/` rather than duplicating prose.
- Internal-only changes (refactors, test-only changes, CI) usually need no doc update — confirm there's a user-visible or contributor-visible effect before touching anything.
- If unsure whether a behavior change is intentional final behavior or still in flux, check recent commits/PR context before documenting it as fact.
