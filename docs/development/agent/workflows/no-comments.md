# No-comments pass

Delete the comments that [the comment policy](../comments.md) does not keep,
and fix the code those comments were excusing.

Adapted from the `no-comments` skill and `comment-sicko` agent in
[cursor/plugins](https://github.com/cursor/plugins/blob/main/pstack/skills/no-comments/SKILL.md)
(MIT, see [the notice](#license)). PlainShelf has no `/how`, `/why`, or
`/architect` skills; their steps are folded in below.

## Scope

Use the files or directories the user or ticket names. Without a target, use
the current diff against `origin/dev`, working tree included. A large target is
split into disjoint packages or directories, one reviewer each.

## Steps

1. **Review in a fresh agent.** Spawn a read-only reviewer per scope part
   (Claude Code: `subagent_type: "Explore"`, and say "do not edit files" in the
   brief). The brief gives the scope and tells it to:
   - read `docs/development/agent/comments.md` and apply it, nothing else;
   - classify every comment as keep, naming the policy item, or kill;
   - for a kill that explains a surprise in our own code or guards a
     suppression of a correctness rule, flag the exact symbol `MUST KILL` with
     the reshape it needs, in one line;
   - before judging a comment marked `IMPORTANT` or "do not remove", or any
     claimed foreign constraint, read the surrounding code and its history
     (`git log -S`, `git blame`, the linked PR or issue) and keep it only if the
     claim is true today on a live path;
   - report touched files, kill count, keeps with their policy item, and
     `MUST KILL` flags, and edit nothing.

   Check `git status` afterward; a reviewer that edited files is a rejected
   report.
2. **Verify the report.** Reject flags outside scope, kills of directives,
   license headers, or contract doc comments, `MUST KILL` reasons that misstate
   the code, and keeps that cite no policy item or whose claim the code refutes.
   Look for suppressions the reviewer missed. Rerun one rejected report with the
   failure named; if the rerun is also rejected, stop and report the scope as
   open.
3. **Delete the accepted kills.** Move any fact that still matters to the place
   the policy names before deleting it. Leave constraint markers in place;
   step 5 handles them.

   Apply the kills with the language's own tokenizer, not by line numbers
   alone: `go/scanner` for Go, a JavaScript parser such as Acorn for TypeScript,
   the SFC parser for Vue. Before deleting, check that every comment in scope is
   classified exactly once, that each deleted line holds only comment text, and
   that no directive falls in a kill range. Reviewer line numbers drift, and a
   missed block is otherwise invisible.
4. **Fix the trivial `MUST KILL` flags; ticket the rest.** In the same change
   as the deletions, fix only the trivial ones: delete the dead path, drop the
   unused parameter, call the real API. Every flag that needs a new shape, such
   as a rename across callers, an extracted function, or a new type, goes into
   one follow-up ticket for the scope, listing each symbol with its reshape, and
   the deletion change links it. That keeps a comment pass reviewable as
   deletions. When the follow-up runs, settle the caller's usage, types, and
   module boundary first, then make the smallest root-cause change; behavior
   must not change, and do not add guards for symptoms.
5. **Constraint markers.** For each "do not remove" style marker, propose the
   cheapest in-scope encoding (type, runtime check, test, or `internal/repocheck`
   rule) and wait for the user's approval. An unattended run, such as a ticket,
   needs that approval written in the ticket. Approved: encode, then delete.
   Otherwise delete, and report the constraint as unenforced.

## Check before finishing

1. Run the checks `judgment.md` lists for every area touched, lint included.
   `unused` fails on a helper whose last caller a fix removed.
2. Confirm no directive was removed:
   `git diff origin/dev | grep -E '^-.*(//go:|//nolint|eslint-|@ts-|prettier-ignore|Code generated)'`
   lists only suppressions the report deleted on purpose.
3. Search `docs/` and `lessons.md` for the touched paths and confirm each cited
   detail still exists somewhere.
4. Report the deletion count, keeps by policy item, `MUST KILL` flags fixed and
   the follow-up ticket for the rest, encodings made, unenforced constraints,
   reruns, and other open work.

## License

The steps above are adapted from cursor/plugins, distributed under this notice:

```text
MIT License

Copyright (c) 2026 Lauren Tan

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
