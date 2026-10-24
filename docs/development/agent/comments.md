# 30 — Code comments

Code carries no comments by default. A name, a type, a test, or a check states
what a comment would, and unlike a comment it fails when it stops being true.
This replaces the earlier rule that comments explain why, briefly.

The policy covers Go, TypeScript, Vue, shell scripts, the `justfile`, and CI
YAML, test files included. Test fixtures under `testdata/` are data and stay as
they are.

## What may stay

Only these survive. When you are unsure whether one applies, delete the comment.

1. **Directives.** Text a tool reads: `//go:build`, `//go:embed`,
   `//go:generate`, `// Code generated … DO NOT EDIT.`, `// prettier-ignore`,
   `// @vitest-environment`, `/// <reference …>`, and lint suppressions that
   pass the check below.
2. **License headers.**
3. **Foreign constraints.** Non-obvious behavior forced by something this
   repository cannot change: a dependency, the Go or browser runtime, Android
   WebView, Wails, Capacitor, SMB, pCloud, or a protocol. State the constraint
   in one or two sentences and name its source (an issue, a spec, a version),
   and confirm it still holds on a live path before keeping it.
4. **Public API contracts.** Doc comments on surfaces other programs consume:
   the gomobile-bound API in `mobile/` and the types that define the on-disk
   shelf format. Other exported Go identifiers are internal to this repository
   and need no doc comment. The HTTP API is pinned by the tests in
   `server/contract/` and their names, not by comments; those tests and the
   `apitest` helpers follow the same rules as any other test code.
5. **Constraint links.** An issue or RFC link explaining a constraint that code
   cannot express.

## What goes

- Narration of what the next lines do, section banners, commented-out code, and
  TODOs. Track a TODO as a ticket.
- Explanations of a surprise in our own code. Delete the comment and reshape the
  code so the behavior is obvious: rename, extract a function, add a type, or
  restructure. If the reshape is outside the current change, delete the comment
  anyway and report the symbol as open work.
- Justifications for a workaround, such as "fine for now" or "too risky to
  change". Fix the root cause within scope, or delete and report it.
- Markers that guard our own code, such as "do not remove", "do not change this
  wording", or "ask X first". Encode the constraint as a type, a runtime check, a
  test, or an `internal/repocheck` rule, then delete the marker. Encoding needs
  the user's approval; without it, delete the marker and report the constraint
  as unenforced.

A long justification is not an exception. Do not shorten a comment that should
go; delete it.

## Lint suppressions

For `//nolint`, `eslint-disable`, `@ts-ignore`, and `@ts-expect-error`, look up
the rule first. Keep the suppression only when the rule is wrong for that line
or is style-only. When the rule catches real bugs or protects correctness or
safety, fix the code and remove the suppression.

A suppression the linter configuration already makes redundant goes without a
code change: the function is in an exclusion list, the path is exempt, or the
linter is not enabled. For Go, prove it by removing the suppression and running
`golangci-lint`; if it then reports, the suppression was live and the rule above
decides. The repository runs no ESLint, so every `eslint-disable` is redundant.

## Where a deleted fact goes

A fact that still matters moves to the place that enforces or records it: a
name or type, a test whose name states the behavior, an `internal/repocheck`
check, `docs/concepts/` for design, [lessons](lessons.md) for agent traps, or
the pull request body for history. When `lessons.md` or a doc cites a file for
the detail, move the detail before deleting the comment.

A name that replaces a comment names the subject and the case, not the reason
or the outcome: the outcome belongs in the assertion message, the reason in the
test that pins it. Go test functions stay within 50 characters and other
declared Go names within 28. `internal/repocheck/name_length_test.go` enforces
both; its list of older long names only shrinks.

## Applying it

New and edited code follows this policy now. Existing code is cleaned in
tracked passes with the [no-comments workflow](workflows/no-comments.md); do
not sweep files outside the change you are making.
