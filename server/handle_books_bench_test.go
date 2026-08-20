package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

// The homepage lists every book, and it asks for include=char_count so it can
// show a length next to each title. ListBooks itself is answered from the
// in-memory book cache, but char_count is not cached: the handler opens the
// current source's meta.json per book. These benchmarks exist to price that
// difference at shelf sizes a user can actually reach, because the cost is
// per-book filesystem round-trips and so scales with the mount's latency, not
// with the size of the response.
//
// Sizes default to 100 and 1000 books; set PLAINSHELF_BENCH_BOOKS to a
// comma-separated list to measure others (10000 takes a minute to stage).
//
//	go test ./server -run '^$' -bench BenchmarkGetBooks -benchtime 20x
//	PLAINSHELF_BENCH_BOOKS=10000 go test ./server -run '^$' -bench BenchmarkGetBooks -benchtime 5x
const benchBooksEnv = "PLAINSHELF_BENCH_BOOKS"

var defaultBenchBookCounts = []int{100, 1000}

// benchSourceID is the single source every staged book carries.
const benchSourceID = "20260101-b1"

func benchBookCounts(b *testing.B) []int {
	b.Helper()

	spec := strings.TrimSpace(os.Getenv(benchBooksEnv))
	if spec == "" {
		return defaultBenchBookCounts
	}

	var counts []int
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		n, err := strconv.Atoi(field)
		if err != nil || n <= 0 {
			b.Fatalf("%s: %q is not a positive book count", benchBooksEnv, field)
		}
		counts = append(counts, n)
	}
	if len(counts) == 0 {
		b.Fatalf("%s is set but lists no counts", benchBooksEnv)
	}
	return counts
}

func BenchmarkGetBooks(b *testing.B) {
	for _, count := range benchBookCounts(b) {
		handler := benchAppWithBooks(b, count)

		for _, variant := range []struct {
			name string
			url  string
		}{
			{name: "plain", url: "/api/shelves/bench_shelf/books"},
			{name: "char_count", url: "/api/shelves/bench_shelf/books?include=char_count"},
		} {
			b.Run(fmt.Sprintf("books=%d/%s", count, variant.name), func(b *testing.B) {
				// One unmeasured request so the first timed iteration is not
				// the one that pays for whatever the handler warms up.
				benchRequest(b, handler, variant.url)

				b.ReportAllocs()
				for b.Loop() {
					benchRequest(b, handler, variant.url)
				}
			})
		}
	}
}

func benchRequest(b *testing.B, handler http.Handler, url string) {
	b.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		b.Fatalf("GET %s: status %d, body %s", url, rec.Code, rec.Body.String())
	}
}

// benchAppWithBooks stages a shelf of count books on disk, then starts an app
// on it and waits out the initial scan, so the benchmark measures the steady
// state a running server serves the homepage from.
func benchAppWithBooks(b *testing.B, count int) http.Handler {
	b.Helper()

	libRoot := b.TempDir()
	writeBenchShelf(b, libRoot, count)

	// Logging is switched off on both the app and the shelf: a benchmark's
	// stdout is a captured pipe, so leaving the per-request log line on would
	// price the pipe rather than the handler.
	silent := logutil.LogConf{LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone}}

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "bench_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot: libRoot,
					Logger:  silent,

					// Both refresh tiers are pushed far beyond any benchmark
					// run, so neither can fire inside a timed loop. This does
					// not by itself keep the first tier out: see
					// drainInitialBookCheck.
					ScanInterval:      "30m",
					BookCheckInterval: "30m",
				},
			},
		},
		StorePath: b.TempDir(),
		Logger:    silent,
	})
	if err != nil {
		b.Fatalf("NewApp: %v", err)
	}
	b.Cleanup(func() {
		if err := app.Close(); err != nil {
			b.Fatalf("Close app: %v", err)
		}
	})

	if err := app.Start(); err != nil {
		b.Fatalf("Start app: %v", err)
	}
	for _, shelfData := range app.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(b.Context()); err != nil {
			b.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}

	handler := app.Handler()
	drainInitialBookCheck(b, handler, libRoot)
	return handler
}

// drainInitialBookCheck runs the per-book staleness check to completion before
// anything is timed.
//
// The initial scan leaves lastBookCheck at its zero value, so however recent the
// cache is, the first ListBooks schedules onlyRefreshBooksInCache — a background
// goroutine that stats every book.json on the shelf. A benchmark that merely
// issues a warmup request and returns leaves that sweep running underneath its
// first timed iterations, and since both variants share this handler, only the
// one that runs first pays for it. Measuring the steady state means getting the
// sweep over with first.
//
// There is no exported way to wait for it, so the wait is on its only
// observable effect: one book's metadata is rewritten before the first request,
// and the sweep is finished once the listing reports the new title. The refresh
// applies every updated entry and stamps lastBookCheck under one write lock, so
// seeing the title also means the interval above is now in force.
func drainInitialBookCheck(b *testing.B, handler http.Handler, libRoot string) {
	b.Helper()

	const drainedTitle = "Bench Book bench-000000 (cache check drained)"

	writeBenchJSON(b, filepath.Join(libRoot, "books", "bench-000000.bookpkg", shelf.BookMetaFile), map[string]any{
		"schema_version": shelf.BookMetaSchemaVersion,
		"id":             "bench-000000",
		"title":          drainedTitle,
		"authors":        []string{"Bench Author"},
		"language":       "zh-TW",
		"cover":          "",
		"current_source": benchSourceID,
	})

	deadline := time.Now().Add(2 * time.Minute)
	for {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/shelves/bench_shelf/books", nil))
		if rec.Code != http.StatusOK {
			b.Fatalf("draining book check: status %d, body %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), drainedTitle) {
			return
		}
		if time.Now().After(deadline) {
			b.Fatal("the initial per-book cache check did not finish within 2 minutes")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// writeBenchShelf writes the book packages directly rather than going through
// the shelf API: staging 10000 books through NewBookWith would spend the whole
// benchmark budget on setup, and the layout it produces is what the scan reads
// back anyway.
func writeBenchShelf(b *testing.B, libRoot string, count int) {
	b.Helper()

	sourceText := strings.Repeat("這是一段測試用的內文。\n", 40)

	for i := range count {
		bookID := fmt.Sprintf("bench-%06d", i)
		bookDir := filepath.Join(libRoot, "books", bookID+".bookpkg")
		sourceDir := filepath.Join(bookDir, shelf.SourcesFolder, benchSourceID)
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			b.Fatalf("MkdirAll %s: %v", sourceDir, err)
		}

		writeBenchJSON(b, filepath.Join(bookDir, shelf.BookMetaFile), map[string]any{
			"schema_version": shelf.BookMetaSchemaVersion,
			"id":             bookID,
			"title":          "Bench Book " + bookID,
			"authors":        []string{"Bench Author"},
			"language":       "zh-TW",
			"cover":          "",
			"current_source": benchSourceID,
		})

		writeBenchJSON(b, filepath.Join(sourceDir, shelf.SourceMetaFile), map[string]any{
			"schema_version": shelf.SourceMetaSchemaVersion,
			"id":             benchSourceID,
			"created_at":     "2026-01-01T00:00:00+08:00",
			"format":         "txt",
			"line_count":     40,
			"char_count":     len([]rune(sourceText)),
		})

		sourcePath := filepath.Join(sourceDir, shelf.SourceFile)
		if err := os.WriteFile(sourcePath, []byte(sourceText), 0o644); err != nil {
			b.Fatalf("WriteFile %s: %v", sourcePath, err)
		}
	}
}

func writeBenchJSON(b *testing.B, path string, value map[string]any) {
	b.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		b.Fatalf("Marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.Fatalf("WriteFile %s: %v", path, err)
	}
}
