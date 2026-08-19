// Command migrate-legacy-sources upgrades a shelf's legacy sources to source
// metadata schema v1, in place.
//
// A legacy source is one written before source metadata owned the content
// format: its meta.json has no schema_version and no format, and its chapters
// come from a split_config the reader applies as it renders. This tool bakes
// that split into the text as Markdown headings and stamps the format onto the
// source, so nothing has to interpret split_config any more.
//
// It is a one-off. Once a shelf has no legacy sources left, the tool has no
// further use — it is not built or shipped with the server.
//
//	migrate-legacy-sources -shelf ./shelf              # report only
//	migrate-legacy-sources -shelf ./shelf -dry-run=false
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofrs/flock"

	"github.com/voilelab/plainshelf/internal/legacyupgrade"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

// libraryLockFile mirrors the shelf's own lock file name.
const libraryLockFile = "app/library.lock"

// booksFolder is the directory every shelf has, used to tell a real shelf from
// a mistyped path.
const booksFolder = "books"

// exit codes
const (
	exitOK          = 0
	exitCouldNotRun = 1
	exitNeedsWork   = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	shelfPath := flag.String("shelf", "", "path to the shelf directory (required)")
	dryRun := flag.Bool("dry-run", true, "report what would change without writing; pass -dry-run=false to apply")
	defaultSplitJSON := flag.String("default-split-config", "",
		`the shelf-wide default split as JSON, for legacy sources that carry none of their own, `+
			`e.g. '{"type":"line_count","line_count":500}'`)
	bookIDs := flag.String("book", "", "comma-separated book ids to limit the run to; default every book")
	verbose := flag.Bool("v", false, "also report sources that need no migration")
	flag.Parse()

	if *shelfPath == "" {
		flag.Usage()
		return exitCouldNotRun
	}

	defaultSplit, err := parseDefaultSplit(*defaultSplitJSON)
	if err != nil {
		return fail("%v", err)
	}
	if err := checkShelfDir(*shelfPath); err != nil {
		return fail("%v", err)
	}

	unlock, err := lockShelf(*shelfPath)
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()

	fmt.Printf("shelf at %s\n", *shelfPath)
	fmt.Printf("default split configuration: %s\n", describeSplit(defaultSplit))
	if defaultSplit.Type != shelf.SplitTypeNone && !legacyupgrade.SplitProducesChapters(defaultSplit) {
		warn("the default split configuration names no chapter boundary, so it splits nothing")
	}
	if *dryRun {
		fmt.Println("dry run: nothing will be written")
	} else {
		fmt.Println("applying changes in place; back up the shelf directory first, and keep PlainShelf closed")
	}
	fmt.Println()

	sh, err := openShelf(*shelfPath)
	if err != nil {
		return fail("%v", err)
	}
	defer sh.Close()

	report, err := legacyupgrade.MigrateShelf(sh, legacyupgrade.Options{
		DefaultSplit: defaultSplit,
		DryRun:       *dryRun,
		BookIDs:      splitBookIDs(*bookIDs),
	})
	if err != nil {
		return fail("%v", err)
	}

	printReport(report, *dryRun, *verbose)
	if !report.Clean() {
		return exitNeedsWork
	}
	return exitOK
}

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "migrate-legacy-sources: "+format+"\n", args...)
	return exitCouldNotRun
}

func warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

// parseDefaultSplit reads the -default-split-config flag.
//
// A legacy source with no split of its own renders through the server's
// default_split_config setting, which lives in the application store rather
// than in the shelf. This tool only opens the shelf, so an operator who has set
// that default has to repeat it here; otherwise those sources migrate as the
// single-chapter text they would be with no default at all.
func parseDefaultSplit(raw string) (shelf.SplitConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return shelf.SplitConfig{}, nil
	}

	var config shelf.SplitConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return shelf.SplitConfig{}, fmt.Errorf("parse -default-split-config: %w", err)
	}
	// "none" and "" are the two spellings of no split; see NormalizeSplitType.
	config = legacyupgrade.NormalizeSplitType(config)
	switch config.Type {
	case shelf.SplitTypeNone, shelf.SplitTypeLineCount, shelf.SplitTypeRegex, shelf.SplitTypeBoundary:
		return config, nil
	default:
		return shelf.SplitConfig{}, fmt.Errorf(
			"parse -default-split-config: unknown split type %q", config.Type)
	}
}

// checkShelfDir refuses a path that is not already a shelf. Opening a shelf
// creates the directory structure, so without this a mistyped path would report
// a clean run over the empty shelf it had just created.
func checkShelfDir(shelfPath string) error {
	info, err := os.Stat(filepath.Join(shelfPath, booksFolder))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s does not look like a shelf: it has no %s directory", shelfPath, booksFolder)
	}
	return nil
}

// lockShelf takes the shelf's own lock for the whole run.
//
// This tool cannot tell whether PlainShelf is running: the server holds the
// shelf lock only for the length of one operation, and nothing else about a
// shelf directory says who else has it open. The lock still stops two
// migrations from racing, and blocks a server that attempts a shelf-level
// operation mid-run — but stopping the server first is the operator's job.
func lockShelf(shelfPath string) (func(), error) {
	lockPath := filepath.Join(shelfPath, filepath.FromSlash(libraryLockFile))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("prepare the shelf lock at %s: %w", lockPath, err)
	}

	lock := flock.New(lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		// Cloud and network mounts do not always support flock. That is a shelf
		// this build can still migrate, just not one it can lock.
		warn("cannot lock the shelf at %s (%v); make sure nothing else has it open", shelfPath, err)
		return func() {}, nil
	}
	if !locked {
		return nil, fmt.Errorf(
			"the shelf at %s is locked by another process; close PlainShelf and try again", shelfPath)
	}
	return func() { lock.Close() }, nil
}

// openShelf opens the shelf for a single migration pass.
func openShelf(shelfPath string) (*shelf.Shelf, error) {
	shelfConf := shelf.ShelfConf{
		LibRoot: shelfPath,
		// This process already holds the shelf lock on its own descriptor, and
		// flock is per open file description: letting the shelf take it again
		// would block against ourselves.
		LockMode: "none",
		// One pass over current data, with no exported cache and no writes into
		// the server's rotating log files.
		ScanInterval:      "0s",
		BookCheckInterval: "0s",
		Logger: logutil.LogConf{
			Level:   "warn",
			Format:  "text",
			LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr},
		},
	}

	sh, err := shelf.NewShelf(&shelfConf)
	if err != nil {
		return nil, fmt.Errorf("open the shelf at %s: %w", shelfPath, err)
	}
	if err := sh.WaitReady(context.Background()); err != nil {
		sh.Close()
		return nil, fmt.Errorf("scan the shelf at %s: %w", shelfPath, err)
	}
	return sh, nil
}

func splitBookIDs(value string) []string {
	var ids []string
	for _, id := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

func describeSplit(config shelf.SplitConfig) string {
	switch config.Type {
	case shelf.SplitTypeNone:
		return "none"
	case shelf.SplitTypeLineCount:
		return fmt.Sprintf("line_count(%d)", config.LineCount)
	case shelf.SplitTypeRegex:
		return fmt.Sprintf("regex(%q)", config.Regex)
	case shelf.SplitTypeBoundary:
		return fmt.Sprintf("boundary(%d lines)", len(config.Boundaries))
	default:
		return string(config.Type)
	}
}

func printReport(report *legacyupgrade.Report, dryRun, verbose bool) {
	for _, result := range report.Results {
		if result.Action == legacyupgrade.ActionAlreadyV1 && !verbose {
			continue
		}
		fmt.Println(describeResult(result))
	}

	counts := report.Counts()
	fmt.Printf("\nbooks=%d sources=%d\n", report.BooksScanned, len(report.Results))
	for _, action := range []legacyupgrade.Action{
		legacyupgrade.ActionRewrote,
		legacyupgrade.ActionStamped,
		legacyupgrade.ActionAlreadyV1,
		legacyupgrade.ActionNeedsAttention,
		legacyupgrade.ActionFailed,
	} {
		if counts[action] > 0 {
			fmt.Printf("  %-16s %d\n", action, counts[action])
		}
	}
	if counts[legacyupgrade.ActionRewrote] > 0 {
		fmt.Println("\nCompare the chapter counts above against what the reader used to show:" +
			" a split regex does not always mean the same thing to Go as it did in the browser.")
	}
	if dryRun {
		fmt.Println("\nDry run: nothing was written. Re-run with -dry-run=false to apply.")
	}
}

// callerPrefixRE matches the fully-qualified function names util.Errorf
// prepends at every wrapping layer. They are useful in a log and pure noise in
// a report an operator reads.
var callerPrefixRE = regexp.MustCompile(`^[^\s:]+\.[^\s:]+: `)

func cleanErrorText(err error) string {
	text := err.Error()
	for {
		trimmed := callerPrefixRE.ReplaceAllString(text, "")
		if trimmed == text {
			return text
		}
		text = trimmed
	}
}

func describeResult(result legacyupgrade.SourceResult) string {
	fields := []string{
		"book=" + result.BookID,
		"source=" + result.SourceID,
		"action=" + string(result.Action),
	}
	if result.Format != "" {
		fields = append(fields, "format="+result.Format)
	}
	if result.Chapters > 0 {
		fields = append(fields, fmt.Sprintf("chapters=%d", result.Chapters))
	}
	if result.Split.Type != shelf.SplitTypeNone {
		fields = append(fields, "split="+describeSplit(result.Split))
	}
	if result.Reason != "" {
		fields = append(fields, fmt.Sprintf("reason=%q", result.Reason))
	}
	if result.Err != nil {
		fields = append(fields, fmt.Sprintf("error=%q", cleanErrorText(result.Err)))
	}
	return strings.Join(fields, " ")
}
