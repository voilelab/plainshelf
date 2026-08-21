package shelf

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

/*
The fingerprint cache is what makes similarity detection affordable to run more
than once: computing a source's fingerprint costs a full read plus a pass over
every character of it, and the answer never changes for the same bytes.

The file has two levels of key, and each one answers a question the other
cannot.

  index    {bookID}/{sourceID} -> the size, mtime and MD5 of that source.txt

    Cheap. One Stat decides whether the record still describes the file, so a
    source that has not changed is answered without opening it at all - which
    is the whole point, because a cache that has to read the file to find out
    whether it can skip reading the file saves nothing.

    Keyed on the book ID rather than on the path because a book ID survives a
    retitle and a move between layers, while the folder name behind it does
    not. Moving a bookpkg with a file manager must not throw its fingerprints
    away; a path key would.

  entries  MD5 of source.txt -> the fingerprint of that exact content

    Immutable. The same bytes always produce the same fingerprint, so an entry
    is never updated, only added. Two copies of the same book therefore share
    one entry, and merging two machines' caches is a set union that cannot
    conflict.

Neither level works alone. Keyed only by content, a lookup would have to read
the whole file to learn the MD5 it needs to look the fingerprint up by - the
deadlock the index level exists to break. Keyed only by stat, a book copied in
or restored from a backup would be fingerprinted again from scratch.

Why this is not stored in book.json or meta.json, which are the files that
already describe a source:

  - Wrong level. A fingerprint belongs to one source's content, and one that
    lived in book.json would have to be re-derived whenever the current source
    pointer moved.
  - Wrong shape. An entry is ~1.4 kB of base64 per source. meta.json is meant
    to stay readable in a text editor.
  - Wrong lifetime. Changing the normalization or the sketch parameters
    invalidates every fingerprint at once. Under app/ that is one file to
    delete; in the books themselves it would be a full-library rewrite, which
    is exactly what docs/concepts/data-format-versioning.md refuses to do.

So this lives under app/ with the other rebuildable state: a missing,
unreadable, too-new or differently-parameterized file is a cache miss and
nothing more.
*/

// fingerprintCacheSchemaVersion is the format this build writes. Like the other
// files under app/ there is no migration path and none is needed: anything this
// build cannot read is discarded and recomputed.
const fingerprintCacheSchemaVersion = 1

// fingerprintCacheFileName is the cache under app/. It is not named per
// installation the way the exported book cache is: a fingerprint is a pure
// function of the content, so every machine computes the same answer and they
// can all share one file. See mergeFingerprintIndex for how they combine.
const fingerprintCacheFileName = "fingerprint-cache.json"

// fingerprintCacheRacyWindow is how recently source.txt may have been written
// before the cache refuses to remember its stat.
//
// This is the same "racily clean" rule the directory scan cache applies, for
// the same reason: timestamps are coarse - whole seconds on ext3 and HFS+, two
// on a FAT-backed SMB share - so a file rewritten inside the tick that was just
// recorded keeps a stat the cache would go on believing. Here the consequence
// is worse than a stale directory listing, because the fingerprint that would
// be served is the previous content's. Leaving such a source out of the index
// costs one re-read on the next run, after which its mtime is old enough to
// record.
const fingerprintCacheRacyWindow = 2 * time.Second

// ErrIncompleteFingerprintAlgo is returned when a caller opens the cache
// without saying which rules its fingerprints were produced by. Every field is
// required: a cache that cannot describe its own algorithm cannot tell a stale
// entry from a usable one.
var ErrIncompleteFingerprintAlgo = util.NewError("fingerprint algorithm must name a normalization, a shingling, a hash and a k")

// FingerprintAlgo names the rules behind every entry in the file.
//
// It is supplied by the caller rather than defined here so that the shelf
// package stays independent of the fingerprinting code: this file knows how to
// store an answer and when to trust it, not how to compute one.
//
// Any difference at all discards the whole file. There is deliberately no
// field-level migration - a changed normalization invalidates the sketch built
// on top of it, so keeping "the parts that did not change" would mean keeping
// entries whose comparability nobody can vouch for.
type FingerprintAlgo struct {
	// Normalize is the canonical-form version, e.g. textnorm.NormalizeVersion.
	Normalize string `json:"normalize"`

	// Shingle describes what was hashed, e.g. "char-5gram".
	Shingle string `json:"shingle"`

	// Hash is the shingle hash version, e.g. textnorm.Hash64Version.
	Hash string `json:"hash"`

	// K is how many hashes a sketch retains.
	K int `json:"k"`
}

func (a FingerprintAlgo) complete() bool {
	return a.Normalize != "" && a.Shingle != "" && a.Hash != "" && a.K > 0
}

// FingerprintEntry is one source's fingerprint, as stored under its content
// hash. It is written once and never modified: the content it describes cannot
// change without changing the key it is stored under.
type FingerprintEntry struct {
	// NormMD5 identifies the normalized text the sketch was built from. Two
	// sources whose raw bytes differ only in layout share it, which is what
	// makes an exact-duplicate check possible without comparing sketches.
	NormMD5 string `json:"norm_md5"`

	// NormChars is the length of the normalized text in runes, and Shingles the
	// number of distinct shingles it held. Both are kept because a similarity
	// report reads them - a 3% overlap means something different between two
	// novels than between a novel and its blurb.
	NormChars int `json:"norm_chars"`
	Shingles  int `json:"shingles"`

	// Sketch is the fingerprint itself, in the encoding the sketch package
	// produces. Opaque here: this package stores it and never interprets it.
	Sketch string `json:"sketch"`
}

// fingerprintIndexEntry is what one Stat has to match for a source to be
// answered without opening it.
type fingerprintIndexEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`

	// MD5 is the hash of the content that stat described, which is the key the
	// fingerprint itself is stored under.
	MD5 string `json:"md5"`
}

func (e fingerprintIndexEntry) matches(stat *FileStat) bool {
	return stat != nil && e.Size == stat.Size && e.ModTime.Equal(stat.ModTime)
}

// fingerprintCacheFile is the on-disk shape of app/fingerprint-cache.json.
//
// Written compact rather than indented, unlike the exported book cache: nothing
// outside PlainShelf reads this file, and its entries are base64 blobs that
// indentation does not make any more readable.
type fingerprintCacheFile struct {
	// SchemaVersion is declared first so it marshals as the first key, keeping
	// the file self-describing (same reasoning as BookMeta.SchemaVersion).
	SchemaVersion int `json:"schema_version"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	Algo    FingerprintAlgo                  `json:"algo"`
	Index   map[string]fingerprintIndexEntry `json:"index"`
	Entries map[string]FingerprintEntry      `json:"entries"`
}

// FingerprintCacheStats records what one run of a fingerprint task cost, so the
// effect of the cache is observable rather than only inferable from wall time.
type FingerprintCacheStats struct {
	// StatHits is how many sources were answered from the index, without being
	// opened at all, and ContentHits how many had to be read but were then
	// recognized by content - a copied, restored or re-imported book.
	StatHits    int
	ContentHits int

	// Built is how many fingerprints had to be computed, and Repaired how many
	// sources had an md5_hash in meta.json that disagreed with their content.
	Built    int
	Repaired int
}

// FingerprintBuilder computes the fingerprint of one source. The cache calls it
// only when neither level of key could answer, and hands it the content it has
// already read so the builder does not read the file a second time.
type FingerprintBuilder func(content []byte) (FingerprintEntry, error)

// FingerprintCache is one process's working copy of app/fingerprint-cache.json.
//
// Open it once per run, Resolve every source through it, then Save. It is safe
// to Resolve from several goroutines at once, which is how a task that walks a
// whole shelf will want to use it.
type FingerprintCache struct {
	shelf *Shelf
	algo  FingerprintAlgo

	mu      sync.Mutex
	index   map[string]fingerprintIndexEntry
	entries map[string]FingerprintEntry
	stats   FingerprintCacheStats
}

// OpenFingerprintCache loads the cache for algo, starting from empty whenever
// the file on disk cannot be used - it is absent, unreadable, written by a
// newer build, or describes different fingerprinting rules.
//
// None of those is an error: the file is derived from the shelf and the run
// about to happen rebuilds it. The one error reported is a caller that did not
// say which rules it fingerprints by, because a cache that cannot answer that
// cannot be safe at all.
func (s *Shelf) OpenFingerprintCache(algo FingerprintAlgo) (*FingerprintCache, error) {
	if !algo.complete() {
		return nil, util.Errorf("%w", ErrIncompleteFingerprintAlgo)
	}

	cache := &FingerprintCache{
		shelf:   s,
		algo:    algo,
		index:   map[string]fingerprintIndexEntry{},
		entries: map[string]FingerprintEntry{},
	}

	filePath := path.Join(appFolder, fingerprintCacheFileName)
	stored, _, err := s.readFingerprintCacheFile()
	switch {
	case err != nil:
		s.Debug("no fingerprint cache to load", "path", filePath, "error", err)
	case stored.Algo != algo:
		// Discarded whole, not merged: see FingerprintAlgo.
		s.Debug("discarding a fingerprint cache built with different rules", "path", filePath, "stored", stored.Algo, "wanted", algo)
	default:
		cache.index = stored.Index
		cache.entries = stored.Entries
		s.Debug("loaded the fingerprint cache", "path", filePath, "index", len(stored.Index), "entries", len(stored.Entries))
	}

	return cache, nil
}

// Stats reports what the run so far cost.
func (c *FingerprintCache) Stats() FingerprintCacheStats {
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
// Paths 2 and 3 read the file, which means they learn the source's true MD5 -
// so they also repair meta.json when the md5_hash stored there disagrees. That
// field is written at import and never validated afterwards, so a source edited
// by hand carries a confidently wrong hash until something reads the content
// again. This is that something.
func (c *FingerprintCache) Resolve(book *Book, source *Source, build FingerprintBuilder) (FingerprintEntry, error) {
	if book == nil || source == nil || build == nil {
		return FingerprintEntry{}, util.NewError("resolving a fingerprint needs a book, a source and a builder")
	}

	key := fingerprintIndexKey(book.ID(), source.ID())

	// Stat before the read, never after. Read the other way round, a source
	// rewritten between the two calls would be recorded with the newer stat and
	// the older content, and every later run would trust it. This order can
	// only pair older stat values with newer content, which merely costs the
	// next run a re-read.
	stat, statErr := source.contentStat()
	if statErr != nil {
		// Not fatal on its own: the read below reports a source that is really
		// gone, with an error that says what was being read.
		c.shelf.Debug("could not stat a source before fingerprinting it", "book_id", book.ID(), "source_id", source.ID(), "error", statErr)
	} else if entry, ok := c.lookupByStat(key, stat); ok {
		return entry, nil
	}

	readAt := time.Now()
	content, err := source.readContent()
	if err != nil {
		return FingerprintEntry{}, util.Errorf("%w", err)
	}

	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(content))
	if err != nil {
		return FingerprintEntry{}, util.Errorf("%w", err)
	}

	c.repairSourceHash(source, md5Hash)

	entry, known := c.lookupByContent(md5Hash)
	if !known {
		entry, err = build(content)
		if err != nil {
			return FingerprintEntry{}, util.Errorf("%w", err)
		}

		// Refused rather than stored: an entry is keyed by the content it
		// describes and is never revisited, so an empty one would be handed
		// out for that content forever, and the only cure would be deleting
		// the file by hand.
		if entry.Sketch == "" {
			return FingerprintEntry{}, util.NewError("a fingerprint builder returned no sketch")
		}
	}

	c.record(key, stat, readAt, md5Hash, entry, known)
	return entry, nil
}

// lookupByStat answers from the index, and only when the entry it names is
// really there: a prune or a hand-edited file can leave an index record
// pointing at content the file no longer holds a fingerprint for.
func (c *FingerprintCache) lookupByStat(key string, stat *FileStat) (FingerprintEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexed, ok := c.index[key]
	if !ok || !indexed.matches(stat) {
		return FingerprintEntry{}, false
	}

	entry, ok := c.entries[indexed.MD5]
	if !ok {
		return FingerprintEntry{}, false
	}

	c.stats.StatHits++
	return entry, true
}

func (c *FingerprintCache) lookupByContent(md5Hash string) (FingerprintEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[md5Hash]
	if ok {
		c.stats.ContentHits++
	}
	return entry, ok
}

// record stores what the run just learned. The entry is stored under its
// content hash whether or not it was just built - it is the same value either
// way - while the index is only extended for a source whose stat is old enough
// to be believed later. See fingerprintCacheRacyWindow.
func (c *FingerprintCache) record(key string, stat *FileStat, readAt time.Time, md5Hash string, entry FingerprintEntry, known bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !known {
		c.entries[md5Hash] = entry
		c.stats.Built++
	}

	if stat == nil || !stat.ModTime.Before(readAt.Add(-fingerprintCacheRacyWindow)) {
		return
	}

	c.index[key] = fingerprintIndexEntry{Size: stat.Size, ModTime: stat.ModTime, MD5: md5Hash}
}

// repairSourceHash rewrites meta.json when its md5_hash disagrees with the
// content that was just read.
//
// Best-effort by design: a read-only shelf reports fsutil.ErrReadOnly here and
// a source whose schema is newer than this build refuses the write, and neither
// is a reason to fail a fingerprint run that has the answer it came for.
func (c *FingerprintCache) repairSourceHash(source *Source, md5Hash string) {
	repaired, err := source.repairContentHash(md5Hash)
	switch {
	case err != nil:
		c.shelf.Debug("could not repair a stale md5_hash", "source", source.FolderPath(), "error", err)
	case repaired:
		c.shelf.Warn("repaired an md5_hash that disagreed with the source content", "source", source.FolderPath())

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
// Whether there is anything to write is decided by comparing the result against
// the bytes already on disk, not by a "something was computed" flag. A run that
// only ever hit the cache still has something to say when a book has since been
// deleted - that is when its records become collectable - and a flag would skip
// exactly that write.
func (c *FingerprintCache) Save() error {
	// Before anything is compared, so that a read-only shelf reports what it is
	// rather than appearing to have saved.
	root, err := c.shelf.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	index := maps.Clone(c.index)
	entries := maps.Clone(c.entries)

	stored, raw, readErr := c.shelf.readFingerprintCacheFile()
	if readErr == nil && stored.Algo == c.algo {
		mergeFingerprintIndex(index, stored.Index)
		mergeFingerprintEntries(entries, stored.Entries)
	} else {
		// Not worth keeping: a file this build cannot read, or one built with
		// other rules, is replaced whole by what this run computed.
		raw = nil
	}

	// Only against a shelf that has finished scanning: an empty book list from
	// a shelf that has not looked yet would prune the whole file.
	if live, ok := c.shelf.liveBookIDs(); ok {
		pruneFingerprintCache(index, entries, live)
	}

	data, err := json.Marshal(fingerprintCacheFile{
		SchemaVersion: fingerprintCacheSchemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Algo:          c.algo,
		Index:         index,
		Entries:       entries,
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	filePath := path.Join(appFolder, fingerprintCacheFileName)
	if !bytes.Equal(data, raw) {
		// Atomic, like every other file under app/: a sync client copying the
		// shelf away must never pick up half a cache.
		if err := fsutil.WriteFileAtomic(root, filePath, data); err != nil {
			return util.Errorf("%w", err)
		}
		c.shelf.Debug("wrote the fingerprint cache", "path", filePath, "index", len(index), "entries", len(entries))
	}

	c.index = index
	c.entries = entries
	return nil
}

// readFingerprintCacheFile reads and validates the file, also returning its raw
// bytes so a save that would not change anything can skip the write.
func (s *Shelf) readFingerprintCacheFile() (*fingerprintCacheFile, []byte, error) {
	file, err := s.dbRoot.Open(path.Join(appFolder, fingerprintCacheFileName))
	if err != nil {
		return nil, nil, util.Errorf("%w", err)
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, util.Errorf("%w", err)
	}

	var stored fingerprintCacheFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, nil, util.Errorf("%w", err)
	}
	if stored.SchemaVersion != fingerprintCacheSchemaVersion {
		return nil, nil, util.Errorf("fingerprint cache is schema_version %d, this build reads %d", stored.SchemaVersion, fingerprintCacheSchemaVersion)
	}
	if stored.Index == nil {
		stored.Index = map[string]fingerprintIndexEntry{}
	}
	if stored.Entries == nil {
		stored.Entries = map[string]FingerprintEntry{}
	}

	return &stored, raw, nil
}

// mergeFingerprintEntries folds other into entries as a union, keeping what this
// run computed wherever both hold the same key.
//
// A union is safe here and nowhere else in the file because an entry is keyed
// by the content it describes: for two machines to disagree about a value they
// would have to disagree about MD5 itself. Which side wins a shared key is
// therefore arbitrary, and the fresher one is the easier to reason about.
func mergeFingerprintEntries(entries, other map[string]FingerprintEntry) {
	for md5Hash, entry := range other {
		if _, ok := entries[md5Hash]; !ok {
			entries[md5Hash] = entry
		}
	}
}

// mergeFingerprintIndex folds other into index, keeping the record that was
// taken later wherever both describe the same source. An index record, unlike
// an entry, is a claim about a file that can have changed since - so the newer
// observation is the one worth keeping.
func mergeFingerprintIndex(index, other map[string]fingerprintIndexEntry) {
	for key, candidate := range other {
		if current, ok := index[key]; ok && !candidate.ModTime.After(current.ModTime) {
			continue
		}
		index[key] = candidate
	}
}

// pruneFingerprintCache drops what the shelf no longer holds: index records for
// books that are gone, and then every entry no surviving record names.
//
// Entries are collected the same way pruneStaleBookCaches works - by what is
// still referenced, not by age - because an entry is only worth keeping while
// some source still hashes to it. A book another machine added since this
// shelf last scanned loses its record here and is fingerprinted again there,
// which costs one recomputation and cannot lose data.
func pruneFingerprintCache(index map[string]fingerprintIndexEntry, entries map[string]FingerprintEntry, live map[string]struct{}) {
	referenced := make(map[string]struct{}, len(index))
	for key, record := range index {
		bookID, _, ok := splitFingerprintIndexKey(key)
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

// liveBookIDs reports the books the shelf currently holds, and false when it
// has not finished its first scan and cannot answer.
func (s *Shelf) liveBookIDs() (map[string]struct{}, bool) {
	if !s.IsReady() {
		return nil, false
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	ids := make(map[string]struct{}, len(s.bookCache.cache))
	for bookID := range s.bookCache.cache {
		ids[bookID] = struct{}{}
	}
	return ids, true
}

func fingerprintIndexKey(bookID, sourceID string) string {
	return bookID + "/" + sourceID
}

func splitFingerprintIndexKey(key string) (bookID, sourceID string, ok bool) {
	bookID, sourceID, ok = strings.Cut(key, "/")
	if !ok || bookID == "" || sourceID == "" {
		return "", "", false
	}
	return bookID, sourceID, true
}
