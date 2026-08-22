package fingerprint

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// racyWindow is how recently source.txt may have been written before the cache
// refuses to remember its stat.
//
// This is the same "racily clean" rule the directory scan cache applies, for
// the same reason: timestamps are coarse - whole seconds on ext3 and HFS+, two
// on a FAT-backed SMB share - so a file rewritten inside the tick that was just
// recorded keeps a stat the cache would go on believing. Here the consequence
// is worse than a stale directory listing, because the fingerprint that would
// be served is the previous content's. Leaving such a source out of the index
// costs one re-read on the next run, after which its mtime is old enough to
// record.
const racyWindow = 2 * time.Second

// indexEntry is what one Stat has to match for a source to be answered without
// opening it.
type indexEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`

	// MD5 is the hash of the content that stat described, which is the key the
	// fingerprint itself is stored under.
	MD5 string `json:"md5"`

	// SeenAt is when these values were FIRST observed, and orders two machines'
	// records of the same source against each other. A run that re-observes the
	// same values carries the original timestamp forward, so an untouched shelf
	// still produces byte-identical files and is never rewritten.
	//
	// The file's own mtime cannot play this part. Restoring a source from a
	// backup gives it new content under an OLDER mtime, and a merge that ranked
	// records by mtime would reinstate the record describing the content that is
	// no longer there - permanently, because every later run would observe the
	// restored file, lose the merge again, and rebuild.
	//
	// Two machines with skewed clocks can still order each other's records
	// wrongly. The cost is bounded as everything here is: a record that loses
	// when it should have won fails its next Stat check and costs one re-read.
	SeenAt time.Time `json:"seen_at,omitzero"`
}

func (e indexEntry) matches(stat *bookpkg.FileStat) bool {
	return stat != nil && e.Size == stat.Size && e.ModTime.Equal(stat.ModTime)
}

// describesSame reports whether two records make the same claim about a file,
// ignoring when each was observed.
func (e indexEntry) describesSame(other indexEntry) bool {
	return e.Size == other.Size && e.ModTime.Equal(other.ModTime) && e.MD5 == other.MD5
}

// supersedes reports whether e is the record worth keeping. Observation time
// decides it; a record written before this build added that field falls back to
// the file's modification time, which is what the older format has to offer.
func (e indexEntry) supersedes(other indexEntry) bool {
	if !e.SeenAt.Equal(other.SeenAt) {
		return e.SeenAt.After(other.SeenAt)
	}
	return e.ModTime.After(other.ModTime)
}

// cacheFile is the on-disk shape of app/fingerprint-cache.json.
//
// Written compact rather than indented, unlike the exported book cache: nothing
// outside PlainShelf reads this file, and its entries are base64 blobs that
// indentation does not make any more readable.
type cacheFile struct {
	// SchemaVersion is declared first so it marshals as the first key, keeping
	// the file self-describing (same reasoning as BookMeta.SchemaVersion).
	SchemaVersion int `json:"schema_version"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	Algo    Algo                  `json:"algo"`
	Index   map[string]indexEntry `json:"index"`
	Entries map[string]Entry      `json:"entries"`
}

// RepairHashFunc is handed the source the cache has just read and the true MD5
// of its content, so a caller that wants to can repair an md5_hash in meta.json
// that has drifted from the bytes on disk. It returns whether it wrote anything,
// which is what lets the run count repairs.
//
// It exists so the cache can stay pure computation over its own file: whether to
// write back to a source, and how, is the shelf's decision, injected here rather
// than performed by this package. A nil hook, or one that reports
// fsutil.ErrReadOnly, simply leaves the source as it is.
type RepairHashFunc func(source *bookpkg.Source, contentMD5 string) (bool, error)

// Config is everything the cache needs from the shelf that owns it. Only Store
// and Algo are required; the rest default to "do nothing", which is what a
// read-only or standalone reader wants.
type Config struct {
	// Store reads and writes the cache's file under app/.
	Store Store

	// Algo names the rules the fingerprints were produced by. An incomplete one
	// is refused by Open.
	Algo Algo

	// Logger receives the cache's diagnostics; pass the shelf's. It must be a
	// real logger - the zero value has no slog.Logger behind it and would panic
	// on first use.
	Logger logutil.Logger

	// LiveBooks reports the book IDs the shelf still holds, so Save can drop
	// records for books that are gone, and false when the caller cannot answer
	// yet - a shelf still scanning - in which case nothing is pruned. Nil never
	// prunes, which is right for a reader that does not own the shelf's lifecycle.
	LiveBooks func() (map[string]struct{}, bool)

	// RepairHash is the write the cache delegates rather than owns; see
	// RepairHashFunc. Nil never repairs.
	RepairHash RepairHashFunc
}

// Cache is one process's working copy of app/fingerprint-cache.json.
//
// Open it once per run, Resolve every source through it, then Save. It is safe
// to Resolve from several goroutines at once, which is how a task that walks a
// whole shelf will want to use it.
type Cache struct {
	store      Store
	algo       Algo
	logger     logutil.Logger
	liveBooks  func() (map[string]struct{}, bool)
	repairHash RepairHashFunc

	mu      sync.Mutex
	index   map[string]indexEntry
	entries map[string]Entry
	stats   Stats

	// resolved is every content hash this run answered for, which survives a
	// prune even while nothing in the index points at it. See record.
	resolved map[string]struct{}
}

// Open loads the cache for cfg.Algo, starting from empty whenever the file on
// disk cannot be used - it is absent, unreadable, written by a newer build, or
// describes different fingerprinting rules.
//
// None of those is an error: the file is derived from the shelf and the run
// about to happen rebuilds it. The one error reported is a caller that did not
// say which rules it fingerprints by, because a cache that cannot answer that
// cannot be safe at all.
func Open(cfg Config) (*Cache, error) {
	if !cfg.Algo.complete() {
		return nil, util.Errorf("%w", ErrIncompleteAlgo)
	}

	cache := &Cache{
		store:      cfg.Store,
		algo:       cfg.Algo,
		logger:     cfg.Logger,
		liveBooks:  cfg.LiveBooks,
		repairHash: cfg.RepairHash,
		index:      map[string]indexEntry{},
		entries:    map[string]Entry{},
		resolved:   map[string]struct{}{},
	}

	stored, _, err := cache.readFile()
	switch {
	case err != nil:
		cache.logger.Debug("no fingerprint cache to load", "file", cacheFileName, "error", err)
	case stored.Algo != cfg.Algo:
		// Discarded whole, not merged: see Algo.
		cache.logger.Debug("discarding a fingerprint cache built with different rules", "file", cacheFileName, "stored", stored.Algo, "wanted", cfg.Algo)
	default:
		cache.index = stored.Index
		cache.entries = stored.Entries
		cache.logger.Debug("loaded the fingerprint cache", "file", cacheFileName, "index", len(stored.Index), "entries", len(stored.Entries))
	}

	return cache, nil
}

// BookSource names a book and the source a coverage count should look for. The
// caller supplies these rather than the cache reading a book list, so the cache
// never has to know how a shelf enumerates its books.
type BookSource struct {
	BookID   string
	SourceID string
}

// CoverageFor counts, over books, how many already have a fingerprint on record
// for the named source, reading only the index and entries already in memory -
// never a source.txt. A book counts as fingerprinted when the cache holds an
// index record for that source and the entry the record names; a cache built
// under different rules was discarded by Open and answers for none, which is
// what tells the UI a rebuild is due.
func (c *Cache) CoverageFor(books []BookSource) Coverage {
	fingerprinted := 0
	for _, book := range books {
		if _, ok := c.Lookup(book.BookID, book.SourceID); ok {
			fingerprinted++
		}
	}

	return Coverage{
		Total:         len(books),
		Fingerprinted: fingerprinted,
		Missing:       len(books) - fingerprinted,
		Algo:          c.algo,
	}
}

// Stats reports what the run so far cost.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// Resolve returns the fingerprint of a source, computing it with build only
// when the cache cannot answer.
//
// The three paths, cheapest first:
//
//  1. The source's stat still matches what the index recorded, and the entry it
//     names is present: answered without opening the file.
//  2. The stat does not match, but the content hashes to an entry that is
//     already stored: the book was copied, moved between shelves, or restored
//     from a backup. One read, no fingerprinting.
//  3. Neither: build is called with the content, and both levels are updated.
//
// Paths 2 and 3 read the file, which means they learn the source's true MD5 - so
// they also offer it to the RepairHash hook, which repairs meta.json when the
// md5_hash stored there disagrees. That field is written at import and never
// validated afterwards, so a source edited by hand carries a confidently wrong
// hash until something reads the content again. This is that something - but the
// write itself belongs to the shelf, not here; see RepairHashFunc.
func (c *Cache) Resolve(book *bookpkg.Book, source *bookpkg.Source, build Builder) (Entry, error) {
	return c.resolve(book, source, build, false)
}

// Rebuild is Resolve with every cache hit ignored: the source is always read
// and always fingerprinted afresh, and both levels of the cache are overwritten
// with the result.
//
// Its reason to exist is that path 1 cannot be trusted to notice every change.
// The index answers from a stat, and a stat is coarse - a source rewritten
// inside racyWindow, or under a filesystem whose clock ticks in whole seconds,
// can keep the size and mtime the index already recorded. When that happens the
// incremental Resolve serves the previous content's fingerprint, and the only
// way back is to read the file regardless of what the stat claims. That is the
// one path this skips.
//
// A needless Rebuild is not a wrong answer, only a wasted one: the entry it
// stores is byte-identical whenever the content really was unchanged, so the
// cost is a read and a fingerprint, never a corrupted cache.
func (c *Cache) Rebuild(book *bookpkg.Book, source *bookpkg.Source, build Builder) (Entry, error) {
	return c.resolve(book, source, build, true)
}

func (c *Cache) resolve(book *bookpkg.Book, source *bookpkg.Source, build Builder, force bool) (Entry, error) {
	if book == nil || source == nil || build == nil {
		return Entry{}, util.NewError("resolving a fingerprint needs a book, a source and a builder")
	}

	key := indexKey(book.ID(), source.ID())

	// Stat before the read, never after. Read the other way round, a source
	// rewritten between the two calls would be recorded with the newer stat and
	// the older content, and every later run would trust it. This order can
	// only pair older stat values with newer content, which merely costs the
	// next run a re-read.
	//
	// Still statted under force, because record needs the stat to refresh the
	// index; only the hit it would produce is ignored, and lookupByStat is not
	// consulted at all so its counter is not touched.
	stat, statErr := source.ContentStat()
	if statErr != nil {
		// Not fatal on its own: the read below reports a source that is really
		// gone, with an error that says what was being read.
		c.logger.Debug("could not stat a source before fingerprinting it", "book_id", book.ID(), "source_id", source.ID(), "error", statErr)
	} else if !force {
		if entry, ok := c.lookupByStat(key, stat); ok {
			return entry, nil
		}
	}

	readAt := time.Now()
	content, err := source.ReadContent()
	if err != nil {
		return Entry{}, util.Errorf("%w", err)
	}

	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(content))
	if err != nil {
		return Entry{}, util.Errorf("%w", err)
	}

	c.repair(source, md5Hash)

	// Under force the content level is skipped too, so a source whose bytes are
	// unchanged is still fingerprinted rather than recognized: record then
	// counts it as Built and overwrites the entry, which is what makes "force
	// recomputed everything" observable.
	var entry Entry
	known := false
	if !force {
		entry, known = c.lookupByContent(md5Hash)
	}
	if !known {
		entry, err = build(content)
		if err != nil {
			return Entry{}, util.Errorf("%w", err)
		}

		// Refused rather than stored: an entry is keyed by the content it
		// describes and is never revisited, so an empty one would be handed
		// out for that content forever, and the only cure would be deleting
		// the file by hand.
		if entry.Sketch == "" {
			return Entry{}, util.NewError("a fingerprint builder returned no sketch")
		}
	}

	c.record(key, stat, readAt, md5Hash, entry, known)
	return entry, nil
}

// lookupByStat answers from the index, and only when the entry it names is
// really there: a prune or a hand-edited file can leave an index record
// pointing at content the file no longer holds a fingerprint for.
func (c *Cache) lookupByStat(key string, stat *bookpkg.FileStat) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexed, ok := c.index[key]
	if !ok || !indexed.matches(stat) {
		return Entry{}, false
	}

	entry, ok := c.entries[indexed.MD5]
	if !ok {
		return Entry{}, false
	}

	c.stats.StatHits++
	return entry, true
}

func (c *Cache) lookupByContent(md5Hash string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[md5Hash]
	if ok {
		c.stats.ContentHits++
	}
	return entry, ok
}

// Lookup returns the fingerprint on record for a source without opening any
// file: it reads only the index and entries already loaded into memory, and ok
// is false unless an index record names the source and the entry that record
// names is present. It is the read side of the cache, for the coverage count and
// the similarity sweep that report what the cache knows rather than recompute
// it.
//
// Unlike lookupByStat it does not compare a stat, and unlike Resolve it never
// falls back to reading the file. Confirming the record still describes the
// current bytes would cost the very Stat the caller is here to avoid; keeping
// the record fresh is the incremental fingerprint task's job, and a source
// changed since it last ran simply reads stale until the next build. Lookups do
// not touch the run's Stats: nothing was resolved.
func (c *Cache) Lookup(bookID, sourceID string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexed, ok := c.index[indexKey(bookID, sourceID)]
	if !ok {
		return Entry{}, false
	}
	entry, ok := c.entries[indexed.MD5]
	if !ok {
		return Entry{}, false
	}
	return entry, true
}

// record stores what the run just learned. The entry is stored under its
// content hash whether or not it was just built - it is the same value either
// way - while the index is only extended for a source whose stat is old enough
// to be believed later. See racyWindow.
func (c *Cache) record(key string, stat *bookpkg.FileStat, readAt time.Time, md5Hash string, entry Entry, known bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !known {
		c.entries[md5Hash] = entry
		c.stats.Built++
	}

	// Pinned whether or not it can be indexed below. An entry nothing points at
	// is normally collectable, but one this run just computed is not garbage: a
	// source too freshly written to index would otherwise have its fingerprint
	// built, saved, dropped by the same Save, and built again next time.
	c.resolved[md5Hash] = struct{}{}

	if stat == nil || !stat.ModTime.Before(readAt.Add(-racyWindow)) {
		return
	}

	observed := indexEntry{Size: stat.Size, ModTime: stat.ModTime, MD5: md5Hash, SeenAt: readAt}
	if current, ok := c.index[key]; ok && current.describesSame(observed) {
		observed.SeenAt = current.SeenAt
	}

	c.index[key] = observed
}

// repair offers a source's just-computed content hash to the RepairHash hook, so
// a stale md5_hash in its meta.json can be corrected by the shelf that owns it.
//
// Best-effort by design: a read-only shelf reports fsutil.ErrReadOnly here and a
// source whose schema is newer than this build refuses the write, and neither is
// a reason to fail a fingerprint run that has the answer it came for. A nil hook
// does nothing at all.
func (c *Cache) repair(source *bookpkg.Source, md5Hash string) {
	if c.repairHash == nil {
		return
	}

	repaired, err := c.repairHash(source, md5Hash)
	switch {
	case err != nil:
		c.logger.Debug("could not repair a stale md5_hash", "source", source.FolderPath(), "error", err)
	case repaired:
		c.logger.Warn("repaired an md5_hash that disagreed with the source content", "source", source.FolderPath())

		c.mu.Lock()
		c.stats.Repaired++
		c.mu.Unlock()
	}
}

// Save merges what this run learned into the file on disk, drops what the shelf
// no longer holds, and writes the result atomically.
//
// The merge happens at write time rather than at load time because the file is
// shared: another machine may have fingerprinted a book of its own since this
// run started, and a plain overwrite would throw its work away.
//
// It is a merge, not a transaction. Two machines saving at the same instant can
// both read the same older file and replace it in turn, and the second write
// then loses what the first added. Serializing that would mean holding the shelf
// lock across a read and a write, which no other file under app/ does and which
// lock_mode: none cannot offer at all. The trade is deliberate: such a race
// costs a fingerprint computed again on some later run, never a wrong answer,
// because an entry is keyed by the content it describes.
//
// Whether there is anything to write is decided by comparing the result against
// the bytes already on disk, not by a "something was computed" flag. A run that
// only hit the cache still has something to say when a book has since been
// deleted - that is when its records become collectable - and a flag would skip
// exactly that write.
func (c *Cache) Save() error {
	// Before anything is compared, so that a read-only shelf reports what it is
	// rather than appearing to have saved.
	if err := c.store.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	index := maps.Clone(c.index)
	entries := maps.Clone(c.entries)

	stored, raw, readErr := c.readFile()
	if readErr == nil && stored.Algo == c.algo {
		mergeIndex(index, stored.Index)
		mergeEntries(entries, stored.Entries)
	} else {
		// Not worth keeping: a file this build cannot read, or one built with
		// other rules, is replaced whole by what this run computed.
		raw = nil
	}

	// Only when the caller can name the live books, and says its list is ready:
	// an empty or absent set from a shelf that has not scanned yet would prune
	// the whole file.
	if c.liveBooks != nil {
		if live, ok := c.liveBooks(); ok {
			pruneCache(index, entries, live, c.resolved)
		}
	}

	data, err := json.Marshal(cacheFile{
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Algo:          c.algo,
		Index:         index,
		Entries:       entries,
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !bytes.Equal(data, raw) {
		if err := c.store.WriteFileAtomic(cacheFileName, data); err != nil {
			return util.Errorf("%w", err)
		}
		c.logger.Debug("wrote the fingerprint cache", "file", cacheFileName, "index", len(index), "entries", len(entries))
	}

	c.index = index
	c.entries = entries
	return nil
}

// readFile reads and validates the cache file, also returning its raw bytes so a
// save that would not change anything can skip the write.
func (c *Cache) readFile() (*cacheFile, []byte, error) {
	raw, err := c.store.ReadFile(cacheFileName)
	if err != nil {
		return nil, nil, util.Errorf("%w", err)
	}

	var stored cacheFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, nil, util.Errorf("%w", err)
	}
	if stored.SchemaVersion != schemaVersion {
		return nil, nil, util.Errorf("fingerprint cache is schema_version %d, this build reads %d", stored.SchemaVersion, schemaVersion)
	}
	if stored.Index == nil {
		stored.Index = map[string]indexEntry{}
	}
	if stored.Entries == nil {
		stored.Entries = map[string]Entry{}
	}

	return &stored, raw, nil
}

// mergeEntries folds other into entries as a union, keeping what this run
// computed wherever both hold the same key.
//
// A union is safe here and nowhere else in the file because an entry is keyed
// by the content it describes: for two machines to disagree about a value they
// would have to disagree about MD5 itself. Which side wins a shared key is
// therefore arbitrary, and the fresher one is the easier to reason about.
func mergeEntries(entries, other map[string]Entry) {
	for md5Hash, entry := range other {
		if _, ok := entries[md5Hash]; !ok {
			entries[md5Hash] = entry
		}
	}
}

// mergeIndex folds other into index, keeping the later observation wherever both
// describe the same source. An index record, unlike an entry, is a claim about a
// file that can have changed since it was made - so the machine that looked most
// recently is the one to believe. See indexEntry.SeenAt for why that is not the
// same as the later mtime.
func mergeIndex(index, other map[string]indexEntry) {
	for key, candidate := range other {
		if current, ok := index[key]; ok && !candidate.supersedes(current) {
			continue
		}
		index[key] = candidate
	}
}

// pruneCache drops what the shelf no longer holds: index records for books that
// are gone, and then every entry no surviving record names.
//
// Entries are collected the way pruneStaleBookCaches does - by what is still
// referenced, not by age - because an entry is only worth keeping while some
// source still hashes to it. A book another machine added since this shelf last
// scanned loses its record here and is fingerprinted again there, which costs
// one recomputation and cannot lose data.
//
// keep holds the content hashes this run answered for, which are exempt: a
// source may deliberately have no index record yet, and its fingerprint is the
// newest thing in the file rather than the stalest.
//
// What this does NOT collect is a record for a source deleted from a book that
// is still there. Deciding that needs the source IDs each book still holds - a
// directory listing per book at save time, the per-book I/O this whole file
// exists to avoid - or a promise from the caller that its run covered every
// source in the shelf. Until a fingerprint task exists to make that promise,
// such a record is left in place: it costs about 1.5 kB and is never served,
// because the source it names is gone.
func pruneCache(index map[string]indexEntry, entries map[string]Entry, live, keep map[string]struct{}) {
	referenced := make(map[string]struct{}, len(index)+len(keep))
	maps.Copy(referenced, keep)
	for key, record := range index {
		bookID, _, ok := splitIndexKey(key)
		if !ok {
			delete(index, key)
			continue
		}
		if _, alive := live[bookID]; !alive {
			delete(index, key)
			continue
		}
		referenced[record.MD5] = struct{}{}
	}

	for md5Hash := range entries {
		if _, ok := referenced[md5Hash]; !ok {
			delete(entries, md5Hash)
		}
	}
}

func indexKey(bookID, sourceID string) string {
	return bookID + "/" + sourceID
}

func splitIndexKey(key string) (bookID, sourceID string, ok bool) {
	bookID, sourceID, ok = strings.Cut(key, "/")
	if !ok || bookID == "" || sourceID == "" {
		return "", "", false
	}
	return bookID, sourceID, true
}
