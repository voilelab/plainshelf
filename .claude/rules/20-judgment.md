# 20 — Scope, judgment, and completion

## Make progress without guessing broadly

Prefer a reversible, repository-consistent choice when the answer can be found
in nearby code, tests, documentation, or history. Ask the user when a choice:

- deletes data or is difficult to reverse;
- changes a public API, shelf format, security boundary, or compatibility promise;
- performs an external action the user did not request;
- has multiple reasonable product or visual interpretations with materially
  different outcomes.

Do not ask about facts that can be verified locally.

## Keep scope controlled

- Change the smallest coherent surface that solves the request.
- Preserve unrelated user changes.
- Do not rewrite tests to bless an implementation unless the previous contract
  is demonstrably wrong and that contract change is explicit.
- If a small task grows into a large cross-module diff, pause and reassess the
  layer where the problem should be solved.

## Signals to change direction

Stop repeating the current approach when any of these occurs:

- the same root-cause error appears again after a corrective attempt;
- fixing one side repeatedly breaks another;
- workarounds begin stacking;
- the diff becomes disproportionate to the requested behavior;
- the only path forward changes an unapproved product or compatibility decision.

Return to the original requirement, identify the invalid assumption, and choose
a smaller experiment, a different design, or a user decision.

## Completion standard

A task is complete only when:

1. the requested behavior or artifact exists;
2. relevant checks pass, or unrun/failed checks are clearly disclosed;
3. generated or edited artifacts have been read back where appropriate;
4. `git diff` contains no accidental or unrelated edits;
5. user-facing docs and release notes are synchronized when required.

Minimum checks by area:

| Area | Minimum check |
|---|---|
| Go | build frontend if needed, then `go test ./...` and `golangci-lint run` |
| Desktop or reader Go | main Go check plus `go test ./...` and `golangci-lint run` inside `desktop` and `reader` |
| Vue/TypeScript | `npm --prefix frontend test`, `npm --prefix frontend run build`, and the `check-boundaries` and `check-licenses` gates |
| UI behavior | relevant Playwright test or an explicitly described manual/browser check |
| Server API | Go tests plus review of the matching `server/contract/<area>` package |
| Documentation/rules | link/build validation and read-back of the diff |

This table is a floor, not a ceiling: every check `.github/workflows/ci.yml`
gates for the area you touched is mandatory before pushing, whether or not it is
listed above. Lint is one of those gates.

Use stronger independent review for security, data migration, deletion, or other
high-impact changes.
