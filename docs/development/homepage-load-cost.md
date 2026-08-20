# Homepage Load Cost

The library homepage is the first thing a user waits for, and two of its costs
were only ever reasoned about from the source: the per-book work behind
`GET /books?include=char_count`, and the two font imports at the top of
`frontend/src/main.ts`. This page records what those two actually cost when
measured, so that any change to either starts from a number rather than from an
argument.

Nothing here proposes a fix. The measurements are reproducible with the commands
below; re-run them before and after any change that claims to improve either
figure.

## What was measured, and on what

Everything below was measured on a 4-vCPU Linux container with an SSD-backed
virtual disk. Absolute timings are specific to that machine — the figures worth
carrying elsewhere are the **ratios** and the **per-book operation counts**, both
of which are properties of the code.

## 1. The cost of `include=char_count`

`GET /books` is answered from the in-memory book cache and touches no file (see
[Shelf Cache and Disk I/O](../concepts/shelf-cache-and-io.md)). `include=char_count`
is not cached: for each book, `getBooks` opens the current source's `meta.json`
and reads `char_count` out of it (`server/handle_books.go`, `shelf/book.go`'s
`GetSource`). So the query parameter converts a memory read into one filesystem
round trip per book.

`BenchmarkGetBooks` in `server/handle_books_bench_test.go` prices it:

```bash
go test ./server -run '^$' -bench BenchmarkGetBooks -benchtime 30x -count 5
PLAINSHELF_BENCH_BOOKS=10000 go test ./server -run '^$' -bench BenchmarkGetBooks -benchtime 10x -count 5 -timeout 30m
```

Medians of five runs, one HTTP request per operation:

| Books | `GET /books` | `+ include=char_count` | Difference | Ratio | Per book |
|---:|---:|---:|---:|---:|---:|
| 100 | 0.19 ms | 3.19 ms | +3.00 ms | 17× | +30.0 µs |
| 1,000 | 1.38 ms | 30.5 ms | +29.1 ms | 22× | +29.1 µs |
| 10,000 | 27.9 ms | 338 ms | +310 ms | 12× | +31.0 µs |

The per-book cost is the stable figure here: about 30 µs at every size. The
ratio falls at 10,000 books only because the plain listing is by then building
an 8.7 MB response and is allocation-bound, which also makes it the noisiest
column (22.6–51.2 ms across the five runs).

Allocation follows the same shape: 5 allocations per book without the parameter,
39 with it.

Both curves are linear in the number of books, so the shelf size a user has
decides whether this matters. At 100 books the difference is invisible. At
10,000 books on a local disk it is a third of a second of server time before a
single byte of the response is written.

### Where the time goes: eight directory opens per book

Counting syscalls tells the more useful story, because it is the part that does
not depend on this machine. The same benchmark was run twice under
`strace -f -c`, once per variant, with identical staging — so the difference is
attributable to `include=char_count` alone:

```bash
go test -c -o /tmp/server.test ./server
PLAINSHELF_BENCH_BOOKS=1000 strace -f -c -U name,calls -o sys-plain.txt \
  /tmp/server.test -test.run '^$' -test.bench 'BenchmarkGetBooks/books=1000/plain$' -test.benchtime 10x
```

Per book, `include=char_count` adds:

| Syscall | Extra calls per book |
|---|---:|
| `openat` | 8.00 |
| `close` | 8.00 |
| `fcntl` | 4.00 |
| `newfstatat` | 1.00 |
| `read` | 1.00 |

Eight opens for one small JSON file is not a bug, it is `os.Root`: it resolves a
path one component at a time so that no lookup can escape the shelf root. The
handler asks for two paths per book, and each is walked from the shelf root
again:

- `Stat("books/{book}.bookpkg/sources/{source}")` — 3 `openat` + 1 `newfstatat`
- `Open(".../{source}/meta.json")` — 5 `openat` + 1 `read`

That is **exactly ten filesystem-facing operations per book**, of which two
concern the file the handler actually wants. The rest re-walk `books/` and the package
directory that the previous book already walked.

### Extrapolating to a high-latency mount

No SMB mount was available in the measurement environment, so the SMB figures
below are **a model, not a measurement**. The model is simple: the operations
above are latency-bound, not throughput-bound, so the added time is

```text
extra time ≈ books × operations-per-book × round-trip-time
```

The model was validated by injecting a fixed delay into exactly those syscalls
with `strace`, which does produce real wall-clock time:

```bash
PLAINSHELF_BENCH_BOOKS=100 strace -f -e trace=openat,newfstatat,read \
  -e inject=openat,newfstatat,read:delay_enter=1000 \
  /tmp/server.test -test.run '^$' -test.bench 'BenchmarkGetBooks/books=100/char_count$' -test.benchtime 3x
```

At 100 books, `include=char_count` cost 318 ms with no injected delay, 673 ms at
250 µs, and 1,475 ms at 1,000 µs. Between the last two that is a slope of 10.7
delayed operations per book, against the ten counted above — the model holds.
`GET /books` without the parameter stayed at 1.4–2.3 ms across all three, which
is the same finding from the other side: it has no per-book I/O to delay.

Taking the counted 10 operations per book as the pessimistic end and 2 as the
optimistic one (an SMB client that caches directory handles pays for `meta.json`
only), the added time is:

| Books | RTT 0.5 ms | RTT 1 ms | RTT 5 ms |
|---:|---:|---:|---:|
| 100 | 0.1–0.5 s | 0.2–1.0 s | 1–5 s |
| 1,000 | 1–5 s | 2–10 s | 10–50 s |
| 10,000 | 10–50 s | 20–100 s | 100–500 s |

The spread between the two columns of each cell is wide, and closing it needs a
real SMB mount. The conclusion does not depend on which end is right: on a
network shelf this is seconds, not milliseconds, and it is the largest known
homepage cost by a wide margin.

## 2. The cost of the two font imports

`frontend/src/main.ts` imports two variable fonts:

```ts
import '@fontsource-variable/noto-serif-tc/wght.css';
import '@fontsource-variable/noto-sans-tc/wght.css';
```

Measured by building twice — once as shipped, once with those two lines deleted —
with `npm --prefix frontend run build`:

| Entry asset | As shipped | Without the imports | Difference |
|---|---:|---:|---:|
| `assets/index-*.js` | 192,164 B (56,210 B gzip) | 192,164 B (56,196 B gzip) | none |
| `assets/index-*.css` | 245,484 B (92,194 B gzip) | 19,751 B (4,462 B gzip) | −225,733 B (−87,732 B gzip) |
| `dist/` total | 12,587,541 B | 2,459,712 B | −10,127,829 B |
| `.woff2` files in `dist/` | 208 (9,902,096 B) | 0 | −208 |

The entry chunk is untouched: the imports are CSS-only. What they add is
**225,733 bytes of `@font-face` declarations — 213 of them, 92% of the entry
CSS** — and that file is render-blocking.

The 9.9 MB of `.woff2` is not on the critical path: `@font-face` declares the
files, the browser fetches only the unicode-range subsets it needs for glyphs it
actually renders. So the question is which pages need them. Loading the mock
frontend in Chromium and counting `.woff2` responses answers it:

| Page | `.woff2` fetched | Bytes |
|---|---:|---:|
| `/` (dashboard) | 0 | 0 |
| `/books` (library) | 0 | 0 |
| `/books/{id}` (book detail) | 4 | 205,104 |

Zero on the homepage, because the global `font-family` in
`frontend/src/styles.css` is `'Avenir Next', 'Segoe UI', sans-serif` — it never
names the Noto families. Those are named in three places only:
`BookDetail.vue`, `BookDetailPage.vue`, and the reader's font options in
`useReaderSettings.ts`.

So the homepage pays for the two imports exactly once: **92 KB gzip of
render-blocking CSS declaring fonts that the homepage does not use**. Whether
that is worth changing is a separate question — the fonts are real dependencies
of the book detail page and the reader, and their licences are listed in the
About panel.

## Reproducing this

```bash
# Server, default sizes 100 and 1000
go test ./server -run '^$' -bench BenchmarkGetBooks -benchtime 30x -count 5

# Server, other shelf sizes (10000 takes about a minute to stage)
PLAINSHELF_BENCH_BOOKS=10000 go test ./server -run '^$' -bench BenchmarkGetBooks \
  -benchtime 10x -count 3 -timeout 30m

# Frontend entry assets
npm --prefix frontend run build
ls -l frontend/dist/assets/index-*.js frontend/dist/assets/index-*.css
```

The benchmark stages a synthetic shelf under `b.TempDir()` — one source per book,
one `meta.json` each — and starts a real app on it, so the timed loop measures
the steady state a running server answers from. Two details are what make it the
steady state:

- **The first per-book cache check is drained before anything is timed.** A
  freshly scanned shelf has never run one, so the first listing schedules a
  background sweep that stats every `book.json`. Left running, it lands
  underneath the first timed iterations of whichever variant runs first — which
  is exactly what an earlier version of this benchmark did, and it is why the
  syscall counts above came out at 8.2 and 1.1 per book instead of 8 and 1.
  `drainInitialBookCheck` waits it out by rewriting one book's title and polling
  the listing until it appears.
- **Logging is switched off** for both the app and the shelf; a benchmark's
  stdout is a captured pipe, and leaving the per-request log line on prices the
  pipe instead of the handler.
