package legacyupgrade

import (
	"errors"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// Action is what the migration did, or would do, to one source.
type Action string

const (
	// ActionRewrote means source.txt was replaced so the legacy split lives on
	// as Markdown headings.
	ActionRewrote Action = "rewrote"
	// ActionStamped means only meta.json changed: the source already reads the
	// way schema v1 says it should.
	ActionStamped Action = "stamped"
	// ActionAlreadyV1 means the source already owns its format.
	ActionAlreadyV1 Action = "already-v1"
	// ActionNeedsAttention means the source was left legacy because this build
	// cannot reproduce its chapters. Nothing was written.
	ActionNeedsAttention Action = "needs-attention"
	// ActionFailed means the source could not be migrated.
	ActionFailed Action = "failed"
)

// Options configures one migration run.
type Options struct {
	// DefaultSplit is the shelf-wide default_split_config, applied to a legacy
	// source that carries no split configuration of its own.
	DefaultSplit shelf.SplitConfig
	// DryRun plans and reports without writing anything.
	DryRun bool
	// BookIDs limits the run to these books. Empty means every book.
	BookIDs []string
}

// SourceResult is what happened to one source.
type SourceResult struct {
	BookID   string
	SourceID string
	Action   Action
	// Format is the format written to meta.json, empty when nothing was written.
	Format string
	// Chapters is how many chapters the migrated source has.
	Chapters int
	// Split is the effective split configuration that was applied.
	Split shelf.SplitConfig
	// Reason explains a non-obvious outcome.
	Reason string
	// Err is why the source was skipped or failed.
	Err error
}

// Report is the outcome of a whole run.
type Report struct {
	Results      []SourceResult
	BooksScanned int
	// BooksCleared lists books whose legacy_source_formats snapshots were
	// dropped because every source now owns its format.
	BooksCleared []string
}

// Counts tallies the results by action.
func (r *Report) Counts() map[Action]int {
	counts := make(map[Action]int)
	for _, result := range r.Results {
		counts[result.Action]++
	}
	return counts
}

// Clean reports whether the run left nothing for an operator to look at.
func (r *Report) Clean() bool {
	counts := r.Counts()
	return counts[ActionNeedsAttention] == 0 && counts[ActionFailed] == 0
}

// MigrateShelf upgrades every legacy source on the shelf to source metadata
// schema v1.
//
// The run is idempotent: a source whose text already carries its chapters is
// only stamped, so re-running after an interruption finishes the job rather
// than splitting the text twice.
func MigrateShelf(sh *shelf.Shelf, opts Options) (*Report, error) {
	books, err := sh.ListBooks()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if len(opts.BookIDs) > 0 {
		wanted := make(map[string]bool, len(opts.BookIDs))
		for _, id := range opts.BookIDs {
			wanted[id] = true
		}
		books = slices.DeleteFunc(books, func(book *shelf.Book) bool {
			return !wanted[book.ID()]
		})
		if len(books) != len(wanted) {
			found := make(map[string]bool, len(books))
			for _, book := range books {
				found[book.ID()] = true
			}
			missing := slices.Sorted(maps.Keys(wanted))
			missing = slices.DeleteFunc(missing, func(id string) bool { return found[id] })
			return nil, util.Errorf("no such book: %s", strings.Join(missing, ", "))
		}
	}

	report := &Report{BooksScanned: len(books)}
	for _, book := range books {
		migrateBook(book, opts, report)
	}
	return report, nil
}

// migrateBook migrates one book's sources and then retires the book-level
// compatibility snapshots the sources no longer need.
func migrateBook(book *shelf.Book, opts Options, report *Report) {
	bookMeta := book.GetMeta()

	sources, err := book.ListSource()
	if err != nil {
		report.Results = append(report.Results, SourceResult{
			BookID: book.ID(), Action: ActionFailed,
			Err: util.Errorf("list sources: %w", err),
		})
		return
	}

	migratedFormats := make(map[string]string)
	for _, source := range sources {
		result := migrateSource(book, bookMeta, source, opts)
		report.Results = append(report.Results, result)
		if result.Action == ActionRewrote || result.Action == ActionStamped {
			migratedFormats[source.ID()] = result.Format
		}
	}

	// Keep book.json's compatibility mirror honest for clients that still read
	// it. New clients take the format from the current source's meta.json.
	if format, ok := migratedFormats[bookMeta.CurrentSource]; ok && format != bookMeta.Format {
		if opts.DryRun {
			bookMeta.Format = format
		} else {
			meta := book.GetMeta()
			meta.Format = format
			if err := book.SetMeta(meta); err != nil {
				report.Results = append(report.Results, SourceResult{
					BookID: book.ID(), SourceID: bookMeta.CurrentSource, Action: ActionFailed,
					Err: util.Errorf("update the book format mirror: %w", err),
				})
				return
			}
		}
	}

	clearLegacySnapshots(book, bookMeta, sources, migratedFormats, opts, report)
}

// clearLegacySnapshots drops book.json's legacy_source_formats once no source
// depends on them any more.
func clearLegacySnapshots(
	book *shelf.Book,
	bookMeta *shelf.BookMeta,
	sources []*shelf.Source,
	migratedFormats map[string]string,
	opts Options,
	report *Report,
) {
	if len(bookMeta.LegacySourceFormats) == 0 {
		return
	}
	for _, source := range sources {
		if _, migrated := migratedFormats[source.ID()]; !migrated && source.IsLegacy() {
			return
		}
	}

	if !opts.DryRun {
		if err := book.ClearLegacySourceFormats(); err != nil {
			report.Results = append(report.Results, SourceResult{
				BookID: book.ID(), Action: ActionFailed,
				Err: util.Errorf("clear legacy_source_formats: %w", err),
			})
			return
		}
	}
	report.BooksCleared = append(report.BooksCleared, book.ID())
}

// migrateSource decides and, unless this is a dry run, performs one source's
// upgrade.
func migrateSource(book *shelf.Book, bookMeta *shelf.BookMeta, source *shelf.Source, opts Options) SourceResult {
	result := SourceResult{BookID: book.ID(), SourceID: source.ID()}

	if !source.IsLegacy() {
		result.Action = ActionAlreadyV1
		return result
	}

	result.Split = effectiveSplit(source.GetMeta().SplitConfig, opts.DefaultSplit)

	content, err := readSource(source)
	if err != nil {
		result.Action = ActionFailed
		result.Err = util.Errorf("read source: %w", err)
		return result
	}

	plan, err := PlanUpgrade(content, result.Split, effectiveFormat(bookMeta, source.ID()))
	if err != nil {
		// The source keeps its legacy metadata and its text. A shelf with a mix
		// of legacy and schema-v1 sources is an ordinary supported state, so
		// this leaves nothing broken behind - only unfinished.
		result.Action = ActionNeedsAttention
		result.Err = err
		result.Reason = reasonFor(err)
		return result
	}

	result.Format = plan.Format
	result.Chapters = plan.Chapters
	result.Reason = plan.Reason
	result.Action = ActionStamped
	if plan.Rewrite {
		result.Action = ActionRewrote
	}

	if opts.DryRun {
		return result
	}

	var newContent io.Reader
	if plan.Rewrite {
		newContent = strings.NewReader(plan.Content)
	}
	if err := source.UpgradeLegacyToSchemaV1(plan.Format, newContent); err != nil {
		result.Action = ActionFailed
		result.Format = ""
		result.Chapters = 0
		result.Err = util.Errorf("%w", err)
	}
	return result
}

// effectiveSplit is the source's own split configuration, falling back to the
// shelf-wide default the reader applies when a legacy source has none.
func effectiveSplit(sourceSplit, defaultSplit shelf.SplitConfig) shelf.SplitConfig {
	if sourceSplit.Type != shelf.SplitTypeNone {
		return sourceSplit
	}
	return defaultSplit
}

// effectiveFormat is the format a legacy source renders as today: its
// book-level snapshot, then book.json's mirror, then plain text.
func effectiveFormat(bookMeta *shelf.BookMeta, sourceID string) string {
	if format := bookMeta.LegacySourceFormats[sourceID]; format != "" {
		return format
	}
	if bookMeta.Format != "" {
		return bookMeta.Format
	}
	return shelf.BookFormatText
}

func readSource(source *shelf.Source) (string, error) {
	file, err := source.Open()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(content), nil
}

// reasonFor turns a skip into something an operator can act on.
func reasonFor(err error) string {
	switch {
	case errors.Is(err, ErrUnsupportedSplitRegex):
		return "split regex cannot run here; convert this source from the source editor"
	case errors.Is(err, ErrSplitRegexNoMatch):
		return "split regex matched no line; check whether the reader still shows chapters"
	case errors.Is(err, ErrUnrecognizedSplitType):
		return "split type is not one this build knows"
	default:
		return ""
	}
}
