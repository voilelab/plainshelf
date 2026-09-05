---
description: "Use when asked to work a tracked ticket end to end: implement it, open the PR, then watch CI and the PR's review comments. Triggers on: 做這張, do this ticket, work this card, a Jira issue URL or key (PSW-…), 發 PR after a ticket."
name: "work-ticket"
tools: [read, bash, grep, glob, edit, write]
argument-hint: "Jira issue URL or key (PSW-…); omit to work the ticket already named in the conversation"
---

Carry one tracked ticket from its card to a reviewed pull request. The phases
are sequential and none is optional: a ticket is not done when the code is
written, it is done when CI is green and the review comments have been read and
answered.

Follow `CLAUDE.md` and `.claude/rules/20-judgment.md` throughout. This skill adds
the surrounding process, not a licence to skip them.

## 1. Read the ticket, then verify it

Fetch the card with the Atlassian MCP tool `getJiraIssue`, passing
`patchouli.atlassian.net` as `cloudId` and the issue key (`PSW-118`) — a browse
URL is the key with a prefix. Ask for `comment` in `fields` as well as the
defaults, because a decision made after the card was written lives there, and
read the description with `responseContentFormat: "markdown"`.

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
  `CLAUDE.md` rule 4.

## 4. Check before pushing

Run every check `.github/workflows/ci.yml` gates for the area you touched. The
minimum-check table in `.claude/rules/20-judgment.md` is the floor and `CLAUDE.md`
spells the commands; neither is a licence to stop before a required gate because
the ticket looked small. Two things neither can tell you, and that are easy to
get wrong:

```sh
npm --prefix frontend run build            # Go builds embed frontend/dist
npm --prefix frontend test                 # the unit suite is separate from the build
npm --prefix frontend run check-boundaries # PR gate
npm --prefix frontend run check-licenses   # PR gate
go test ./... && golangci-lint run
(cd desktop && go test ./... && golangci-lint run)
(cd reader  && go test ./... && golangci-lint run)
```

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
`.claude/rules/50-lessons.md` and run it on a Node version
`docs/development/setup.md` supports.

Then the paperwork: `update-changelog` for `CHANGELOG.md` if the change is
user-visible, `update-docs` for `docs/`. Read back `git diff` for edits you did
not intend.

## 5. Open the pull request

Base `dev`. Body in 繁體中文, code and commit message in English, structured as:

- **問題** — the symptom and the traced cause, in the repository's own terms.
- **改法** — what changed and why this layer rather than another.
- **驗收** — the card's acceptance criteria, each mapped to how it is verified.
- **檢查** — commands run, and explicitly what was not run.
- A link back to the ticket.

Claim only what the change actually does. If the fix is a step rather than a
complete answer to the card's premise, say so in the body under its own heading;
a reviewer who discovers that themselves has been misled by you.

## 6. Watch CI and the review

**There is no `gh` in the cloud container**, and no `hub` either; GitHub is
reached through the `mcp__github__*` tools, loaded by name with `ToolSearch`
(`select:mcp__github__pull_request_read`). A `gh` command copied from another
project's guide will not run here, whatever it promises.

Do not poll. `subscribe_pr_activity` on the new PR — the harness normally asks
for this the moment a PR is created — and then end the turn: a finished check
suite and a submitted review each arrive as a wake event. Ending the turn *is*
how you wait. A `Bash` `sleep` loop over check status is both slower and blind
to the reviews.

On each wake, read the whole PR state. **All three of these are needed:**

```text
pull_request_read  method: "get_check_runs"      # the individual CI jobs
pull_request_read  method: "get_reviews"         # submitted reviews
pull_request_read  method: "get_review_comments" # inline findings, with thread ids
```

`get_reviews` misses inline comments entirely: an automated reviewer's findings
live only in `get_review_comments`, and its summary review body says nothing but
"here are some suggestions". A ticket reported as reviewed on the strength of
`get_reviews` alone has not been reviewed. Page with `perPage`/`after` for the
same reason: a review long enough to spill to a second page is read as a short
one otherwise.

Two things that read as failures and are not. `get_status` answers `pending`
with `total_count: 0` because this repo posts check *runs* and no legacy commit
statuses — read `get_check_runs`, not that. And a wake whose event body echoes a
comment you posted yourself is not a request; skip it.

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
`add_reply_to_pull_request_comment` takes the numeric comment id from the
`#discussion_r…` anchor, `resolve_review_thread` takes the `PRRT_…` thread id.
When a finding is real but outside this ticket, say so on its thread rather than
growing the diff, and carry it into the final report so it can become its own
card.

## 7. Report and stop

Report: what changed, what the checks said, what the review said and what you
did about it. Then stop.

Do not merge the pull request and do not transition the issue in Jira. Both are
the user's to do; offer, and wait to be asked. When asked,
`getTransitionsForJiraIssue` names the transition ids — `Done` is not a status
you can set directly.
