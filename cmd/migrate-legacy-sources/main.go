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
//	migrate-legacy-sources -conf config.yaml              # report only
//	migrate-legacy-sources -conf config.yaml -dry-run=false
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"

	"github.com/voilelab/plainshelf/internal/legacyupgrade"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

// settingKeyDefaultSplitConfig mirrors server.settingKeyDefaultSplitConfig.
const settingKeyDefaultSplitConfig = "default_split_config"

// libraryLockFile mirrors the shelf's own lock file name.
const libraryLockFile = "app/library.lock"

// exit codes
const (
	exitOK          = 0
	exitCouldNotRun = 1
	exitNeedsWork   = 2
)

// migrateConf is the slice of the server's config file this tool reads. It is
// declared here rather than imported because server.AppConf reaches the
// embedded frontend, which is a build artifact this tool has no use for. The
// field types come from the same packages the server uses, and
// TestExampleConfigDecodes guards the key names against drift.
type migrateConf struct {
	AppConf struct {
		Shelves            []*shelf.ShelfConfWithID `yaml:"shelves"`
		StorePath          string                   `yaml:"store_path"`
		DefaultSplitConfig *shelf.SplitConfig       `yaml:"default_split_config"`
	} `yaml:"app_conf"`
}

func main() {
	os.Exit(run())
}

func run() int {
	confPath := flag.String("conf", "", "path to the server config file (required)")
	dryRun := flag.Bool("dry-run", true, "report what would change without writing; pass -dry-run=false to apply")
	shelfID := flag.String("shelf", "", "shelf id from app_conf.shelves; optional when the config declares one shelf")
	bookIDs := flag.String("book", "", "comma-separated book ids to limit the run to; default every book")
	verbose := flag.Bool("v", false, "also report sources that need no migration")
	flag.Parse()

	if *confPath == "" {
		flag.Usage()
		return exitCouldNotRun
	}

	conf, err := loadConf(*confPath)
	if err != nil {
		return fail("%v", err)
	}
	shelfConf, err := selectShelf(conf, *shelfID)
	if err != nil {
		return fail("%v", err)
	}

	// Opening the store doubles as the check for a running PlainShelf: its
	// directory lock is held for as long as a server or desktop app is up.
	db, err := openStore(conf.AppConf.StorePath)
	if err != nil {
		return fail("%v", err)
	}
	if db != nil {
		defer db.Close()
	} else {
		warn("no store_path in the config, so a running server cannot be detected that way")
	}

	unlock, err := lockShelf(shelfConf)
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()

	defaultSplit := resolveDefaultSplit(db, conf.AppConf.DefaultSplitConfig)

	fmt.Printf("shelf %s at %s\n", shelfConf.ID, shelfConf.LibRoot)
	fmt.Printf("default split configuration: %s\n", describeSplit(defaultSplit))
	if defaultSplit.Type != shelf.SplitTypeNone && !legacyupgrade.SplitProducesChapters(defaultSplit) {
		// Most often this is a default_split_config written in the config file:
		// shelf.SplitConfig has no YAML tags, so line_count does not survive the
		// decode. The server reads it the same way, so the migration still
		// matches what the reader shows — but say so rather than let an operator
		// assume their configured default was applied.
		warn("the default split configuration names no chapter boundary, so it splits nothing;\n" +
			"         a line_count set in the config file is not read (only a stored setting is)")
	}
	if *dryRun {
		fmt.Println("dry run: nothing will be written")
	} else {
		fmt.Println("applying changes in place; back up the shelf directory first, and keep the server stopped")
	}
	fmt.Println()

	sh, err := openShelf(shelfConf)
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

func loadConf(confPath string) (*migrateConf, error) {
	raw, err := os.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var conf migrateConf
	if err := yaml.Unmarshal(raw, &conf); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &conf, nil
}

func selectShelf(conf *migrateConf, shelfID string) (*shelf.ShelfConfWithID, error) {
	shelves := conf.AppConf.Shelves
	switch {
	case len(shelves) == 0:
		return nil, errors.New("the config declares no shelves under app_conf.shelves")
	case shelfID == "" && len(shelves) == 1:
		return shelves[0], nil
	case shelfID == "":
		return nil, fmt.Errorf("the config declares %d shelves; choose one with -shelf %s",
			len(shelves), strings.Join(shelfIDs(shelves), "|"))
	}

	for _, candidate := range shelves {
		if candidate.ID == shelfID {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("no shelf %q in the config; it declares %s",
		shelfID, strings.Join(shelfIDs(shelves), ", "))
}

func shelfIDs(shelves []*shelf.ShelfConfWithID) []string {
	ids := make([]string, 0, len(shelves))
	for _, candidate := range shelves {
		ids = append(ids, candidate.ID)
	}
	return ids
}

// openStore opens the server's settings store. A locked store means a
// PlainShelf process is running, which this tool must not race: it rewrites
// source files the server takes no lock on, and opening a second shelf clears
// the staging directory a running server creates books in.
func openStore(storePath string) (*store.DB, error) {
	if storePath == "" {
		return nil, nil
	}
	if _, err := os.Stat(storePath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	db, err := store.New(storePath)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot open the store at %s, which usually means PlainShelf is running; "+
				"stop the server or desktop app and try again (%w)", storePath, err)
	}
	return db, nil
}

// lockShelf takes the shelf's own lock for the whole run.
//
// This is the weaker of the two guards: a running server only holds the shelf
// lock for the length of one operation, so an idle server is not detected here.
// What it does buy is mutual exclusion for the run itself - a server that tries
// a shelf-level operation while the migration is working will block rather than
// interleave - and it stops two migrations from running at once. The store lock
// above is what actually detects a running PlainShelf.
func lockShelf(conf *shelf.ShelfConfWithID) (func(), error) {
	if conf.LockMode == "none" {
		warn("lock_mode is \"none\" for this shelf, so it cannot be locked against another process")
		return func() {}, nil
	}

	lockPath := filepath.Join(conf.LibRoot, filepath.FromSlash(libraryLockFile))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("prepare the shelf lock at %s: %w", lockPath, err)
	}

	lock := flock.New(lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("lock the shelf at %s: %w", conf.LibRoot, err)
	}
	if !locked {
		return nil, fmt.Errorf(
			"the shelf at %s is locked by another process; stop the server or desktop app and try again",
			conf.LibRoot)
	}
	return func() { lock.Close() }, nil
}

// openShelf opens the shelf for a single migration pass.
func openShelf(conf *shelf.ShelfConfWithID) (*shelf.Shelf, error) {
	shelfConf := conf.ShelfConf
	// This process already holds the shelf lock on its own descriptor, and
	// flock is per open file description: letting the shelf take it again would
	// block against ourselves.
	shelfConf.LockMode = "none"
	// One pass over current data, with no exported cache and no writes into the
	// server's rotating log files.
	shelfConf.ScanInterval = "0s"
	shelfConf.BookCheckInterval = "0s"
	shelfConf.BookCacheWriterID = ""
	shelfConf.Logger = logutil.LogConf{
		Level:   "warn",
		Format:  "text",
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr},
	}

	sh, err := shelf.NewShelf(&shelfConf)
	if err != nil {
		return nil, fmt.Errorf("open the shelf at %s: %w", conf.LibRoot, err)
	}
	if err := sh.WaitReady(context.Background()); err != nil {
		sh.Close()
		return nil, fmt.Errorf("scan the shelf at %s: %w", conf.LibRoot, err)
	}
	return sh, nil
}

// resolveDefaultSplit reads default_split_config exactly as the server does:
// the stored setting when there is a usable one, the configured value
// otherwise. A stored value that no longer parses counts as absent.
func resolveDefaultSplit(db *store.DB, configured *shelf.SplitConfig) shelf.SplitConfig {
	if db != nil {
		raw, exists, err := db.GetSetting(settingKeyDefaultSplitConfig)
		switch {
		case err != nil:
			warn("cannot read the %s setting: %v", settingKeyDefaultSplitConfig, err)
		case exists:
			var stored shelf.SplitConfig
			if err := json.Unmarshal(raw, &stored); err != nil {
				warn("the stored %s setting is not valid JSON, ignoring it: %v",
					settingKeyDefaultSplitConfig, err)
			} else {
				return stored
			}
		}
	}
	if configured != nil {
		return *configured
	}
	return shelf.SplitConfig{}
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
	if len(report.BooksCleared) > 0 {
		fmt.Printf("  legacy_source_formats cleared on %d book(s)\n", len(report.BooksCleared))
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
