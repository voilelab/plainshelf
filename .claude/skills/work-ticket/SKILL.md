---
description: "Use when asked to work a tracked ticket end to end: implement it, open the PR, then watch CI and the PR's review comments. Triggers on: 做這張, do this ticket, work this card, a Notion work-item URL, 發 PR after a ticket."
name: "work-ticket"
tools: [read, bash, grep, glob, edit, write]
argument-hint: "Ticket URL or ID (Notion work item); omit to work the ticket already named in the conversation"
---

Carry one tracked ticket from its card to a reviewed pull request. The phases
are sequential and none is optional: a ticket is not done when the code is
written, it is done when CI is green and the review comments have been read and
answered.

Follow `CLAUDE.md` and `.claude/rules/20-judgment.md` throughout. This skill adds
the surrounding process, not a licence to skip them.

## 1. Read the ticket, then verify it

Fetch the card with the Notion MCP tool (`notion-fetch`) using its URL. Read the
whole card, including its properties: `主要檔案` names where to start and `優先度`
and `類別` say how it was triaged.

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
  it without one. Fix the shared helper in `server/contract/apitest_env_test.go`
  rather than each call site, and check the recorders that bypass it
  (`getWailsLike`).
- Read `server/contract/api_*_contract_test.go` for the routes you touch, per
  `CLAUDE.md` rule 4.

## 4. Check before pushing

Which checks apply is decided by the minimum-check table in
`.claude/rules/20-judgment.md`, and the commands are the ones in `CLAUDE.md`.
Two things that table cannot tell you, and that are easy to get wrong:

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

Watch the checks to completion rather than polling by hand — a `Monitor` over
`gh pr checks <n> --json name,bucket` that emits each check as it settles and
exits when none is pending.

Then read the review. **Both of these are needed:**

```sh
gh pr view <n> --json reviews,comments,reviewDecision,state
gh api --paginate repos/voilelab/plainshelf/pulls/<n>/comments   # inline comments
```

The first misses inline comments entirely: an automated reviewer's findings live
only in the second, and its summary review body says nothing but "here are some
suggestions". A ticket reported as reviewed on the strength of the first command
alone has not been reviewed. `--paginate` matters for the same reason: without
it a review long enough to spill to a second page is read as a short one.

**An empty review is not the same as no findings.** The automated reviewer
submits after CI, so reading immediately behind the checks can find nothing and
be wrong minutes later. Wait for a submitted review to appear, and if you stop
without one, report that the review had not landed rather than that it was
clean. A review can also arrive after the PR is merged — read it anyway and
carry anything real into a follow-up.

Verify each finding against the code before accepting or dismissing it — a
throwaway `_test.go` that probes the claim and is deleted afterwards settles it
faster than argument. Bots are right often enough to check and wrong often
enough not to obey. When a finding is real but outside this ticket, say so, and
propose it as its own card rather than growing the diff.

## 7. Report and stop

Report: what changed, what the checks said, what the review said and what you
did about it. Then stop.

Do not merge the pull request and do not change the ticket's `狀態` in Notion.
Both are the user's to do; offer, and wait to be asked.
