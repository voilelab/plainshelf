# Desktop Memory Probe

`memprobe.yml` measures how much memory the Wails desktop app uses on macOS. It
runs when dispatched by hand, and on a pull request that changes the probe
itself. It is not a required check: it reports numbers and sets no threshold.

## What it does

1. Builds the frontend, `plainshelf-srv`, and the desktop app with the
   `memprobe` build tag on a `macos-15` runner.
2. Seeds a shelf through the server's import API with
   `scripts/memprobe/seed.py`: one large Markdown book with many `##` chapters
   plus many ordinary books, each with a cover, spread over ten folders.
3. Points the desktop app's `shelves.json` at that shelf and launches it.
4. The `memprobe` build injects `desktop/memprobe.js` once the page is ready.
   It idles, then repeats a cycle: scroll the library, read through the large
   book chapter by chapter, and open dozens of other books in turn. It marks
   each phase through the bound `MemprobeMark` method, which also records the
   Go runtime's heap figures.
5. `scripts/memprobe/sample.sh` records the RSS of the app process and every
   `com.apple.WebKit.*` process every few seconds, plus the physical footprint
   reported by `vmmap`, and takes a screenshot at each phase change.
6. `scripts/memprobe/report.py` writes the per-phase table to the job summary.

Release builds do not include the probe: without the build tag,
`domReadyProbe` stays nil and `MemprobeMark` is not bound.

## Run it

Dispatch **Desktop memory probe** from the Actions tab, or:

```bash
gh workflow run memprobe.yml --ref <branch> -f cycles=5 -f books=500
```

The inputs set the number and size of books, the number of cycles, and the
idle and settle times. The `memprobe-<run id>` artifact holds `samples.csv`,
`marks.tsv`, `report.md`, the screenshots, and the app's logs.

## Reading the result

- **PlainShelf** is the app process: the Go server and the WKWebView host.
  Rendering and JavaScript run in `WebContent`, a separate WebKit process that
  Activity Monitor lists on its own.
- Compare the `cycle N settle` rows. Memory that keeps rising from one cycle
  to the next is a leak. A level that stays flat after the first cycle is
  caches warming up.
- If the screenshots show an empty window or the marks stop at `idle`, the
  scenario did not run, and the numbers only describe an idle app.
