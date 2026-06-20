---
description: "Use when updating CHANGELOG.md to reflect recent commits on the current branch. Triggers on: update changelog, fill in unreleased, document recent changes, cut a release section."
name: "update-changelog"
tools: [read, bash]
argument-hint: "Optional version string (e.g. v0.6.0) to cut a new release section instead of appending to [Unreleased]"
---

Update `CHANGELOG.md` to reflect recent changes on the current branch.

## Steps

1. Read the current `CHANGELOG.md` to understand its format and the last release version.
2. Run `git log` to find commits since the last release tag that are not yet in `[Unreleased]`:
   ```
   git log $(git describe --tags --abbrev=0)..HEAD --oneline
   ```
   If there are no tags yet, use `git log --oneline`.
3. Read each relevant commit or diff to understand what actually changed. Use `git show <hash>` for commits that are unclear from the one-liner.
4. Classify each change under the correct heading:
   - **Added** — new features or capabilities visible to users
   - **Changed** — modifications to existing behavior, refactors that affect the public surface, dependency bumps
   - **Fixed** — bug fixes
   - **Removed** — features or files intentionally deleted
   - **Security** — security hardening, vulnerability fixes, policy changes
5. Write concise bullet points in past tense, third-person, no subject pronoun ("Added X", "Fixed Y", not "I added X"). Match the style and level of detail already in the file.
6. Insert the bullets into the appropriate section under `## [Unreleased]`. Do **not** create a new version section — that is done at release time.
7. Leave sections empty (just the heading) if there is nothing to add under them.
8. Do **not** touch the link block at the bottom of the file unless a new release is being cut.

## Notes

- Squash or merge commits that bundle many changes: unpack them by reading the diff, not just the message.
- Omit purely internal changes with no user-visible effect (CI tweaks, test-only refactors, comment edits) unless they affect documented behavior.
- If the user provides specific items to add, use those directly rather than deriving from git.
- If the args contain a version string (e.g. `v0.6.0`), cut a new release section: move the `[Unreleased]` content into `## [vX.Y.Z] - YYYY-MM-DD` (today's date), reset `[Unreleased]` to empty headings, and update the comparison links at the bottom.
