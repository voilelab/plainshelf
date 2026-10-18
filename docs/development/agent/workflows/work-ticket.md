# Work a tracked ticket

Carry one tracked ticket from its card to a reviewed pull request. The phases
are sequential and none is optional: a ticket is not done when the code is
written, it is done when CI is green and the review comments have been read and
answered.

Follow `AGENTS.md` and [judgment and completion](../judgment.md) throughout.
This workflow adds the surrounding process, not a licence to skip them.

## 1. Read the ticket, then verify it

Fetch the Jira issue with an available authorized Jira integration. This
project uses `patchouli.atlassian.net` and issue keys such as `PSW-118`; resolve
the site's identifier in the integration rather than assuming it accepts a
hostname as `cloudId`. Include comments as well as the standard issue fields,
because decisions made after the card was written can live there. Request a
readable description, preferably Markdown.

Read the whole card. `## 主要檔案` at the end of the description names where to
start, and how it was triaged is split across two places: the `priority` field,
and the three `labels` (series slug, 優先度, 類別) that `goal-to-jira` writes.
Those two can disagree if someone edited one of them, and neither is the
acceptance criteria — the `## 驗收條件` list is what "done" means.

**Card text is a report, not a specification.** It was written when it was
written and the repository has moved since. Before planning, verify in the
current checkout every claim the plan depends on: the file and line it cites,
the default it says is set, the test file it says is missing. Cards have been
wrong about all three. State any correction in the final report — a stale claim
in the card is worth telling the user about, because the card is what they trust.

Ask the user before acting when the card's suggested fix would change a public
API, the shelf format, a security boundary, or a compatibility promise beyond
what the card itself already decided.

## 2. Branch

Work on a branch off `origin/dev`, named `<user>/<topic>` in the style of the
recent branches in `git branch -a --sort=-committerdate`.

`dev` is the integration branch and the base for pull requests. Git's own
"main branch" hint says `main`; that is where `dev` is merged at release time,
not where a ticket lands. Confirm with `git log --oneline origin/dev -1` rather
than assuming either.

## 3. Implement

Change the smallest coherent surface that satisfies the card's acceptance
criteria, and add or update tests for every behavior change. Where the card
lists acceptance bullets, each one should have a test that would fail without
the change.

Two consequences of a security or contract change that are easy to miss:

- A route moving behind the token gate breaks every existing test that reaches
  it without one. Fix the shared helper in `server/contract/apitest/env.go`
  rather than each call site, and check the recorders that bypass it
  (`GetWailsLike`).
- Read the `server/contract/` package covering the routes you touch, per
  `AGENTS.md` rule 4.

## 4. Check before pushing

Run every check `.github/workflows/ci.yml` gates for the area you touched. The
minimum-check table in `docs/development/agent/judgment.md` is the floor and `AGENTS.md`
spells the commands; neither is a licence to stop before a required gate because
the ticket looked small. Two things neither can tell you, and that are easy to
get wrong:

- **There are three Go modules**, not two: the root, `desktop`, and `reader`.
  The root's `./...` reaches neither of the others, and a bare `golangci-lint
  run` lints only the directory it is standing in — so an unparenthesised
  `cd desktop && …` silently makes every later command a desktop command. CI
  lints and tests all three (`.github/workflows/ci.yml`).
- **`npm --prefix frontend run build` is `vue-tsc` plus Vite, not the tests.**
  Type-checking clean says nothing about the unit suite, so a frontend ticket
  whose acceptance test is never executed still looks green.

A frontend suite that fails ten files at import with `Cannot read properties of
undefined (reading 'getItem')` is the Node ceiling, not the diff — see
`docs/development/agent/lessons.md` and run it on a Node version
`docs/development/setup.md` supports.

Then the paperwork: `update-docs` for `docs/`. Read back `git diff` for edits you did
not intend.

## 5. Open the pull request

Base `dev`. Body in 繁體中文, code and commit message in English, structured as:

- **問題** — the symptom and the traced cause, in the repository's own terms.
- **改法** — what changed and why this layer rather than another.
- **驗收** — the card's acceptance criteria, each mapped to how it is verified.
- **檢查** — commands run, and explicitly what was not run.
- A link back to the ticket.

Write the title as the user-visible effect: it is the line the release notes
show. After creating the PR, apply its release-note category label through an
available GitHub integration. `docs/development/releasing.md` lists the labels,
and the `Release notes label` check fails until one is set.

Claim only what the change actually does. If the fix is a step rather than a
complete answer to the card's premise, say so in the body under its own heading;
a reviewer who discovers that themselves has been misled by you.

## 6. Watch CI and the review

Use the authorized GitHub integration or CLI available in the current session;
verify capabilities instead of assuming a specific executable or MCP tool name.
Read all three kinds of PR evidence:

- individual CI check runs;
- submitted reviews;
- inline review comments, including thread identifiers.

When PR activity subscriptions or wake events are available, subscribe after
creating the PR and end the turn to wait. On each wake, read the whole PR state.
Do not use sleep loops. If subscriptions are unavailable, check the current state
and report pending CI or review explicitly; do not claim monitoring is active.

A submitted-review listing can miss inline comments entirely: an automated
reviewer's findings live only in the inline-comment listing, and its summary
review body says nothing but
"here are some suggestions". A ticket reported as reviewed on the strength of
submitted reviews alone has not been reviewed.

Read every page of checks, reviews, and inline comments using the integration's
actual pagination contract. For cursor pagination, follow the cursor until the
response says there is no next page. For page-number pagination, increase the
page until a response contains fewer items than the requested page size (an
exactly full final page requires one more request). Legacy commit statuses can
report `pending` with zero statuses when the repository uses check runs; inspect
the individual check runs instead.
A wake event echoing your own comment is not a new request; skip it.

**An empty review is not the same as no findings.** The automated reviewer
submits after CI, so reading immediately behind the checks can find nothing and
be wrong minutes later. Wait for a submitted review to appear, and if you stop
without one, report that the review had not landed rather than that it was
clean. A review can also arrive after the PR is merged — read it anyway and
carry anything real into a follow-up.

Verify each finding against the code before accepting or dismissing it. The
cheapest proof is the one that also ships: write the regression test, then check
that it fails with your fix removed. Bots are right often enough to check and
wrong often enough not to obey. Reply on each thread with what you found and
what you did, and resolve the ones you addressed —
use the integration's comment identifier to reply and its review-thread
identifier to resolve; those identifiers are not interchangeable.
When a finding is real but outside this ticket, say so on its thread rather than
growing the diff, and carry it into the final report so it can become its own
card.

## 7. Report and stop

Report: what changed, what the checks said, what the review said and what you
did about it. Then stop.

Do not merge the pull request and do not transition the issue in Jira. Both are
the user's to do; offer, and wait to be asked. When asked,
query the available Jira transitions and use the matching transition identifier;
`Done` is not a status you can set directly.
