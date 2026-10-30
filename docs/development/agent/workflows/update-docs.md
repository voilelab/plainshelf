# Update documentation

Synchronize user and contributor documentation with verified behavior in the
current checkout. For agent guidance, workflows, and tool adapters, follow the
shared [maintenance policy](../maintenance.md) instead.

## Documentation map

| Audience or topic | Canonical location |
|---|---|
| Repository summary and entry links | `README.md` |
| Documentation paths and product boundaries | `docs/index.md` |
| Release installation and upgrades | `docs/installation.md` |
| First library after installation | `docs/getting-started.md` |
| Local and SMB storage configuration | `docs/configuring-*.md` |
| Durable storage/architecture concepts | `docs/concepts/*.md` |
| Source, desktop, Android, and Docker development | `docs/development/*.md` |
| Current limitations | `docs/known-issue.md` |
| Site navigation | `mkdocs.yml` |

`README.md` is intentionally thin. Put procedural detail in `docs/` and link to
it instead of maintaining a second copy.

## Workflow

1. Determine the requested scope. If none is given, inspect commits and the diff
   since the latest release tag or the merge base with `origin/main`.
2. Read the existing canonical page for each affected behavior.
3. Verify commands, flags, config fields, defaults, paths, and UI behavior in
   current code or tests. Do not document from commit messages alone.
4. Make the smallest coherent update. Create a new page only when the topic has
   a distinct audience or lifecycle; add it to `mkdocs.yml` in the same change.
5. Preserve the pre-1.0 warning and product boundaries in `docs/index.md`.
6. Run the [unslop workflow](unslop.md) over the prose you changed.
7. Run `mkdocs build --strict`, check local links, and read back the diff.
8. Report anything that could not be verified instead of guessing.

## Style

- Write task-oriented headings and put prerequisites before commands.
- Keep one canonical procedure; cross-link related pages.
- Use relative links within repository Markdown.
- Distinguish local development defaults from production guidance.
- Omit internal refactors unless they change a user or contributor workflow.
