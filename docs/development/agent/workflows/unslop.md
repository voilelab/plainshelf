# Unslop prose

Edit English prose to remove the patterns that mark text as machine-written,
keeping its meaning and tone. This covers `docs/`, the agent guidance, code
comments, commit messages, and pull request titles and bodies.

Adapted from the `unslop` skill in
[cursor/plugins](https://github.com/cursor/plugins/blob/main/pstack/skills/unslop/SKILL.md)
(MIT, see [the notice](#license)). Rule numbers match upstream, so a gap is a
rule this repository does not use. Rules marked *PlainShelf* differ from
upstream or are added here.

## Scope

1. Edit only the text the user named. Without a target, edit the prose that the
   current diff adds or changes; do not sweep the repository in one change.
2. Leave these unchanged: identifiers, string literals, UI copy, error messages
   that tests or users match on, test fixtures, command output, quoted upstream
   text, and anything in CJK. User replies stay in Traditional Chinese per
   `AGENTS.md`; this workflow is for English text.
3. A rewrite must keep every fact. When a sentence turns out to carry nothing,
   delete it rather than rephrase it.

## Content

3. **Superficial -ing phrases.** "highlighting…", "ensuring…", "reflecting…"
   tacked onto a sentence. Delete them, or state the consequence as its own
   sentence.
5. **Vague attributions.** "Experts believe", "it is widely known". Name the
   source (a file, test, issue, or measurement) or delete the claim.

## Language

7. **AI vocabulary.** additionally, crucial, delve, enduring, enhance, foster,
   garner, interplay, intricate, landscape, pivotal, showcase, tapestry,
   testament, underscore, vibrant, seamless, robust. Use the plain word.
8. **Fancy ways to say "is".** "serves as", "stands as", "boasts", "features".
   Say "is" or "has".
9. **"Not just X, but Y."** State the point directly.
10. **Rule of three.** Use the natural number of items, not three by habit.
11. **Synonym cycling.** Pick one term for one thing and repeat it. In this
    repository that matters doubly: shelf, book, package, and folder are
    defined terms (`docs/concepts/data-model.md`).
12. **False ranges.** "from X to Y" where X and Y are not on one scale. List
    the items.

## Style

13. **Em dashes, sparingly** (*PlainShelf*). Upstream bans them; here, use at
    most one per paragraph, and never where a period or comma does the job. A
    dash that opens an aside followed by a second dash that closes it is the
    usual offender: make the aside its own sentence.
14. **Colon as connector.** A colon may introduce a list, an example, or a
    definition. It should not join two clauses that read fine as two sentences.
15. **Boldface overuse.** Bold a term where it is defined, not every mention.
16. **Inline-header lists.** "**Performance:** Performance improved…" restates
    the label; write prose. A bold lead-in that names the item and is followed
    by new detail is fine, which is the shape `lessons.md` uses.
17. **Sentence-case headings.** "Data format versioning", not "Data Format
    Versioning". Proper nouns keep their capitals. Change the matching
    `mkdocs.yml` nav title in the same edit, and check that no link points at
    the old anchor text (`mkdocs build --strict`).
18. **Decorative emojis.** Remove them from headings and bullets.
19. **Curly quotes.** Use straight quotes in prose and comments. Do not touch
    curly quotes inside string literals or fixtures (Scope, rule 2).

## Communication artifacts

20. **Chatbot phrases.** "I hope this helps", "Let me know if…", "Certainly!",
    "Found the smoking gun!". Remove.
22. **Sycophantic tone.** "Great question", "You're absolutely right". Respond
    directly.

## Filler

23. **Filler phrases.** "In order to" is "to", "due to the fact that" is
    "because". Delete "it is important to note that".
24. **Excessive hedging.** One qualifier at most: "may", not "could potentially".
25. **Generic conclusions.** "This keeps things simple." State the specific
    fact, or end without a summary.

## Jargon

26. **Abstract metaphor nouns.** substrate, wedge, vector (as "way"), locus,
    nexus, bedrock, scaffolding, paradigm, north star, flywheel, gold-plating,
    ratchet, endgame, and "surface" or "harness" used as metaphors. Use the
    concrete word: "the routes this package serves", not "the API surface".
    Keep established technical terms (*PlainShelf*): "attack surface",
    "test harness" for an actual harness, and "primitive" when it names Reka UI
    primitives.

## Plain speech

27. **Say what it does, not how it feels.** Name the mechanism or the number.
    If the sentence could appear unchanged in another project's docs, it says
    nothing about this one; cut it. Aphoristic openers ("Your files outlive the
    program that wrote them.") fail this test.
28. **One idea per sentence.** Split a sentence the reader has to parse twice.
29. **Active voice.** Name the actor: "the scanner rewrites `book.json`", not
    "`book.json` is rewritten". Passive is fine when the actor is unknown or
    irrelevant.
30. **Adverbs.** Replace "significantly faster" with the measured delta, and a
    weak verb plus adverb with a stronger verb.
31. **Plain words.** "use" for utilize or leverage, "help" for facilitate,
    "many" for numerous, "if" for "in the event that".
32. **Mannered prose.** Aphorisms, rhetorical fragments, personified code ("the
    lock refuses"), and figurative verbs ("rides along", "carries off"). Say it
    literally.
33. **Over-compression.** Arrows, dropped articles, and verbless fragments make
    the reader decode. "Parser rejects bad date → exit 2" becomes "The parser
    rejects a bad date and exits with code 2." This applies to `lessons.md`
    too; [maintenance](../maintenance.md) gives its sentence format.

## PlainShelf additions

34. **Pull request titles are release notes.** Write the user-visible effect,
    not the implementation ([releasing](../../releasing.md)). "Keep reading
    progress when a book moves between folders", not "Refactor transfer path".
35. **Comments explain why, briefly.** A comment states what the code cannot:
    the reason, the constraint, the trap. Drop narration of what the next lines
    do, and keep a block to the shortest form that keeps every reason. Match
    the comment density of the surrounding file.

## Check before finishing

1. Read the diff with `git diff` and confirm every removed line's fact survives
   somewhere, or was deliberately cut as empty.
2. For `docs/`, run `mkdocs build --strict` and follow the documentation lesson
   on reading removed lines (`lessons.md`, Documentation).
3. For Go or TypeScript comments, run the area's lint and format checks from
   `judgment.md`; a comment edit can still break `gofmt` or a doc-comment lint.

## License

The rules above are adapted from cursor/plugins, distributed under this notice:

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
