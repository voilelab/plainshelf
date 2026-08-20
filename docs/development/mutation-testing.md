# Mutation Testing

A green test suite proves the code ran, not that anything was checked. Mutation
testing asks the sharper question directly: it introduces small breaking changes
into the implementation ("mutants") and reports the ones no test noticed. Every
surviving mutant marks behavior that is executed but not verified.

This page describes the pilot configuration. It covers
`frontend/src/features/reader/utils/` and the shared Markdown and safe-HTML
modules that were split out of it — `markdownAssetImages.ts`,
`markdownChapters.ts`, `markdownLineSyntax.ts`, `renderMarkdownBlocks.ts` and
`safeHtml.ts`, all under `frontend/src/utils/`. It is **not** wired into CI and
has no score threshold.

## Run it

```bash
npm --prefix frontend ci
npm --prefix frontend run test:mutation
```

Stryker instruments through Babel 8, which needs Node.js 22.18 or newer. That is
the floor recorded in the [setup prerequisites](setup.md#prerequisites), because
`npm ci` installs the dependency whether or not the mutation run is used.

The run takes roughly three minutes and writes
`frontend/reports/mutation/mutation.html` (a browsable report) plus
`mutation.json`. Both are ignored by Git.

Line coverage for the same files, for comparison:

```bash
npm --prefix frontend exec -- vitest run --coverage \
  --coverage.provider=v8 \
  --coverage.include='src/features/reader/utils/**' \
  --coverage.include='src/utils/markdown*.ts' \
  --coverage.include='src/utils/renderMarkdownBlocks.ts' \
  --coverage.include='src/utils/safeHtml.ts' \
  --coverage.exclude='**/*.test.ts' --coverage.all
```

Configuration lives in `frontend/stryker.conf.json`. Widening `mutate` to the
whole frontend is not practical: seven files already cost minutes per run.

## Reading a survivor

A surviving mutant is a question, not a defect. Answer it in one of four ways,
and record the answer:

| Answer | Meaning | What to do |
|---|---|---|
| **Add an assertion** | The mutant describes a real behavior nothing checks | Write a test that fails for the mutant. State in one sentence what error it blocks. |
| **Equivalent mutant** | The change cannot alter observable behavior, or only makes it stricter | Record the reasoning. Do not write a test. |
| **Not the unit suite's job** | Real behavior, but owned by an end-to-end test, the type system, or accepted as-is | Record the owner. |
| **Tool artifact** | The suite does catch it; the runner reported it wrongly | Verify by hand (see below), then record it. |

Two rules keep the exercise honest:

- Never weaken or delete an existing assertion to raise the score.
- Never add an assertion that cannot be tied to a concrete wrong behavior.
  `toBeDefined()`, a bare `toBeTruthy()`, and an unreviewed snapshot kill
  mutants without checking anything.

The score is a symptom, not the goal. A score raised by relabelling mutants as
equivalent is worth nothing.

### Known limitation: static mutants

A mutant in module-level code — a top-level `const` regex, an option object, a
call made at import time — runs once when the module is first imported. Under
the Vitest runner these are reported unreliably: the module can stay cached
across mutant activations, so the mutated line never re-executes and the mutant
is reported as *Survived* even though the suite would fail on it.

Two of the survivors in this pilot are exactly that (`markdown.disable([...])`
in `renderMarkdownBlocks.ts`; applying either mutation by hand makes the test
file fail to load). Before accepting a survivor in module-level code, apply the
mutation to the source, run the tests, and revert.

## Pilot results

Measured on `frontend/src/features/reader/utils/`, which then held all seven
files (Node 22,
`concurrency: 2`, `coverageAnalysis: perTest`, 753 mutants, ~3 minutes per run).
"Before" is the state the pilot found; "after" is the current run.

| File | Line coverage before → after | Mutation score before → after |
|---|---|---|
| `markdownAssetImages.ts` | 100% → 100% | 83.13% → 92.77% |
| `markdownChapters.ts` | 98.18% → 98.18% | 70.41% → 88.17% |
| `markdownLineSyntax.ts` (was `parseMarkdownBlocks.ts`) | 73.94% → 100% | 46.60% → 95.16% |
| `mobileReaderGestures.ts` | 90.47% → 100% | 73.68% → 98.68% |
| `parseReaderBlocks.ts` | 0% → 100% | 0.00% → 84.21% |
| `renderMarkdownBlocks.ts` | 94.59% → 98.64% | 47.87% → 84.04% |
| `sanitizeReaderHtml.ts` (now `safeHtml.ts`) | 100% → 100% | 52.90% → 92.70% |
| **All files** | **85.17% → 99.18%** | **55.17% → 89.91%** |

The two columns disagree most where it matters most: `sanitizeReaderHtml.ts` had
every line covered and barely half its behavior verified.

The pilot added 61 tests and changed no implementation code. 225 mutants moved
from surviving to killed, and 217 remained.

Later work has moved those numbers twice, and only one of the two is an
improvement. Removing the unreachable `'span'` entry from `ALLOWED_ATTR` took its
mutant away with the line, which is one fewer survivor in code that is still
there.

**The other is not an improvement at all.** `markdownLineSyntax.ts` is the
pilot's `parseMarkdownBlocks.ts` after the dead block model it also held was
deleted (see its disposition below). Nothing about the surviving code is better
verified than it was: the same three mutants survive it now as then. Its score
rose, and carried the totals and the whole-suite mutant count (985 → 753) with
it, because code no consumer reached left the denominator. Read that row as a
smaller module, not a stronger one — the 140 undisposed-of mutants it used to
contribute are gone along with the module, leaving 76 dispositioned below.

## Standing dispositions

Counts are from the current run. `E` = equivalent mutant, `N` = not the unit
suite's job, `T` = tool artifact. Line numbers refer to the implementation file.

### `parseReaderBlocks.ts` — 6

| Lines | # | | Reasoning |
|---|---:|---|---|
| 7 | 3 | E | The `!text.trim()` fast path is redundant: `.filter(Boolean)` over the trimmed chunks already yields `[]` for a whitespace-only section. |
| 20 | 2 | E | `quoteLines.length > 0` cannot be false. The chunk reached this line through `.filter(Boolean)` after `.trim()`, so it holds at least one non-empty line. |
| 31 | 1 | E | Dropping `^` from `/^>\s?/` changes nothing: the replace is non-global, so it rewrites the first match, and every line here starts with `>`. |

### `safeHtml.ts` (was `sanitizeReaderHtml.ts`) — 10

Line numbers here are the file's before it moved to `src/utils/` and grew the
profile allowlists; the code each row describes is unchanged.

| Lines | # | | Reasoning |
|---|---:|---|---|
| 43 | 1 | E | Emptying `'tbody'` in `ALLOWED_TAGS` leaves a full table byte-identical after sanitization (verified by hand). |
| 85–87 | 3 | E | `colon < 0` is subsumed by the property-name comparison beside it, and trimming the value duplicates what CSSOM does when the value is assigned. |
| 107 | 1 | E | `/\s+/` → `/\s/` only introduces empty strings into the split, and the empty string is not a reader class. |
| 119 | 1 | N | `ALLOW_UNKNOWN_PROTOCOLS: false` has no reachable effect while no URL-bearing attribute is allowed. Kept as defense in depth for a future `ALLOWED_ATTR` change. |
| 123–125, 128 | 4 | E | `style` and `template` content is dropped by the tag rules regardless. `noscript` and `embed` content is never parsed as a child of those elements, so `FORBID_CONTENTS` cannot act on it — it leaks with or without the entry (verified by hand). |

### `mobileReaderGestures.ts` — 1

| Lines | # | | Reasoning |
|---|---:|---|---|
| 32 | 1 | E | `deltaX < 0` and `deltaX <= 0` differ only at zero, which the enclosing `absX >= 60` branch makes unreachable. |

### `markdownAssetImages.ts` — 6

| Lines | # | | Reasoning |
|---|---:|---|---|
| 25–26 | 2 | E | `linkify` and `typographer` change inline rendering only; this parser instance is used solely for line maps. |
| 31 | 3 | N | Separating `&&` from `||` on the angle-bracket check needs a destination with one bracket at one end and a valid asset path in between (`"assets/map.png>`). No Markdown writer emits that; both realistic one-sided spellings are already rejected. Pinning the contrived input would document the mutant, not the behavior. |
| 64 | 1 | E | `line < token.map[1]` → `<=` marks one extra line as inline: the line immediately after an inline block. That line is always blank, a fence marker, or an HTML tag, and none of them can match `IMAGE_LINE_RE`. Argued, not exhaustively proven. |

### `markdownChapters.ts` — 20

| Lines | # | | Reasoning |
|---|---:|---|---|
| 27 | 2 | E | Narrowing the bold capture leaves the stray `**` for the italic pass, which removes it. Both passes together produce the same visible title. |
| 58–59, 150 | 4 | N | `/\r$/` → `/\r/` and the empty replacement differ only for a bare CR inside a line — classic Mac line endings, which this reader does not support. Offsets are unaffected either way. |
| 70 | 1 | E | `offset < chapters[0].startOffset` → `<=` differs only when an H1 line starts exactly where the first H2 starts, which is the same line. |
| 103 | 1 | E | `chapterIndex === 0` → `true` is guarded by the `sections.length === 0` beside it, which is false after the first chapter is pushed. |
| 125–139 | 12 | E | The remaining `findMarkdownEditorSection` mutants all leave the search falling through to `return sections[sections.length - 1]`, which is the answer they would have returned anyway. The offset is clamped to the last section's end, so the loop always resolves and the trailing fallback is unreachable. |

### `renderMarkdownBlocks.ts` — 30

| Lines | # | | Reasoning |
|---|---:|---|---|
| 35 | 2 | T | `markdown.disable(['link', 'autolink'])`. Applying either mutation by hand makes the module throw at import and the test file fail to load. Reported as surviving because the module stays cached. |
| 52–59 | 11 | N | `readerEnvironment` normalizes markdown-it's untyped `env`. `renderMarkdownBlocks` is the only entry point and always passes a well-formed environment, so the fallback is unreachable from the module's public surface. It stays because the `env` type is `unknown`. |
| 66 | 2 | N | The empty-prefix and null-`src` guards in `assetFromToken` are unreachable for the same reason. |
| 79–80 | 3 | E | markdown-it always places the `inline` token next to its `paragraph_open`/`paragraph_close`, so the type check beside it cannot fail. |
| 137 | 2 | E | An image token always carries a `src` attribute. |
| 158–160, 186 | 5 | E | `!source.trim()` versus `!source`: whitespace-only input reaches the same empty result one step later, through the `html.trim()` check. |
| 166 | 1 | E | The document serial only has to be unique between renders; incrementing or decrementing both satisfy that. |
| 175 | 1 | E | Assigning `fence = transition.state` unconditionally is identical, because `updateMarkdownFenceState` returns the current state on a non-boundary line. |
| 190 | 3 | E | `/^[ \t]*/` always matches, so the `?? ''` fallback is unreachable and dropping the anchor matches at the same position. |

### `markdownLineSyntax.ts` — 3

`parseMarkdownHeadingLine` and `updateMarkdownFenceState` are used by the chapter
scanner, the reader's renderer, and the source converter. The module scores what
its callers need:

| Lines | # | | Reasoning |
|---|---:|---|---|
| 14, 15 | 3 | E | Dropping `$` from a regex applied to a single line changes nothing (`.` never matches a newline), and narrowing `[ \t]+` to `[ \t]` is neutralized by the `.trim()` applied to the captured title. |

The file used to be `parseMarkdownBlocks.ts` and also held a **block model** —
`parseMarkdownBlocks()`, `parseTextSegmentToBlocks`, `parseInlineSegments`,
`isMarkdownFenceLine`, and the constants serving them — with no production
caller: the reader renders Markdown through `renderMarkdownBlocks` and plain text
through `parseReaderBlocks`, and every importer took only the line helpers and
the re-exported asset helpers. It contributed 140 undisposed-of mutants, 79 of
them with no covering test at all. Adding tests would have pinned behavior
nothing consumes, so it was deleted instead and the file renamed to what remains
of it.

### Totals

Of the current run's 753 mutants, 676 are killed and one times out. The remaining
76 are the dispositions above:

| | Count |
|---|---:|
| Equivalent mutant | 53 |
| Not the unit suite's job | 21 |
| Tool artifact | 2 |

The pilot's tally of 225 newly killed mutants stays as recorded above: it is a
before/after measurement of that pilot, and part of it fell inside the module
since deleted, so it does not reconcile against this run. What the deletion
removed from the run is 232 mutants of code nothing called, 140 of which no test
had answered for.
