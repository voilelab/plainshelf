package shelf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/internal/textnorm"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

/*
The fingerprint cache holds one similarity fingerprint per distinct source text,
so that the sweep which builds them only ever reads a file it has not seen
before.

It is keyed twice, and neither key alone would do.

The index is keyed by {bookID}/{sourceID} and records the size, modification
time and MD5 of that source's source.txt. It is what makes a repeat sweep cheap:
one Stat answers whether the file is still the one the recorded MD5 describes,
without opening it. Book IDs survive a retitle and a move between layers, so
reorganizing a shelf does not invalidate anything - a path would.

The entries are keyed by that MD5, which is a property of the content and not of
where it lives. Two copies of the same book therefore share one entry, a book
that moves keeps its fingerprint, and two machines sharing a shelf can merge
their caches by taking the union: an entry can never disagree with another entry
under the same key. Keying only by MD5 would deadlock instead - finding the entry
would require the hash, and the hash would require reading the whole file, which
is the cost the cache exists to avoid.

Like every other file under app/ this is runtime state: it is rewritten whole and
atomically, any reader treats a missing, unreadable, too-new or differently
computed file as an empty cache, and deleting it only costs one rebuild.
*/

// fingerprintCacheSchemaVersion is the format this build writes. As with the
// other caches under app/ there is no migration path and none is needed.
const fingerprintCacheSchemaVersion = 1

// FingerprintCacheFileName is the snapshot under app/.
//
// It is not named per installation, unlike the exported book cache. A
// fingerprint is a pure function of the bytes it was computed from, so two
// machines produce byte-identical entries and have nothing to keep apart.
const FingerprintCacheFileName = "fingerprint-cache.json"

// FingerprintAlgo records how every fingerprint in the file was produced.
//
// It is compared as a whole and any difference discards the entire file rather
// than migrating a field: a cache holding two algorithms is worse than no cache,
// because nothing about it looks wrong. Keeping fingerprints under app/ instead
// of in meta.json is what makes that affordable - an algorithm change rewrites
// one file rather than every book in the shelf, which is the trickle-not-rewrite
// rule docs/concepts/data-format-versioning.md chose deliberately.
type FingerprintAlgo struct {
	Normalize string `json:"normalize"`
	Shingle   string `json:"shingle"`
	Hash      string `json:"hash"`
	K         int    `json:"k"`
}

// currentFingerprintAlgo describes what this build computes. Every field is
// taken from the package that owns it, so a version bump there invalidates the
// cache here without anyone having to remember to edit this function.
func currentFingerprintAlgo() FingerprintAlgo {
	return FingerprintAlgo{
		Normalize: textnorm.NormalizeVersion,
		Shingle:   "char-" + strconv.Itoa(sketch.DefaultN) + "gram",
		Hash:      textnorm.Hash64Version,
		K:         sketch.DefaultK,
	}
}

// FingerprintIndexEntry is what one Stat of a source.txt has to match for its
// fingerprint to be reused.
//
// It carries the same limitation as Book.IsStale: a file whose content changes
// while its size and modification time stay identical is not noticed. The
// consequence here is only a missed pair of similar books, never a damaged
// shelf, and a forced rebuild is the escape hatch.
type FingerprintIndexEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	MD5     string    `json:"md5"`
}

// matches reports whether stat describes the same file this entry was recorded
// from.
func (e FingerprintIndexEntry) matches(stat *FileStat) bool {
	return e.Size == stat.Size && e.ModTime.Equal(stat.ModTime)
}

// FingerprintEntry is one document's fingerprint, keyed by the MD5 of the exact
// bytes it was computed from.
type FingerprintEntry struct {
	// NormMD5 identifies the normalized text. Two sources that agree here hold
	// the same content under different layout, which is an exact answer no
	// sketch comparison has to be run for.
	NormMD5 string `json:"norm_md5"`

	// NormChars is the length of the normalized text in runes, and Shingles the
	// exact number of distinct shingles in it. Both are stored because a
	// comparison uses them and neither can be recovered from the sketch.
	NormChars int `json:"norm_chars"`
	Shingles  int `json:"shingles"`

	// Sketch is a sketch.Sketch in its encoded form.
	Sketch string `json:"sketch"`
}

// FingerprintCache is the on-disk shape of app/fingerprint-cache.json.
//
// It is a plain value with no lock of its own: a sweep holds one, mutates it as
// it goes, and hands it to SaveFingerprintCache. Do not share one between
// goroutines.
type FingerprintCache struct {
	// SchemaVersion is declared first so it marshals as the first key, keeping
	// the file self-describing when opened in a text editor.
	SchemaVersion int `json:"schema_version"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	Algo FingerprintAlgo `json:"algo"`

	// Index maps "{bookID}/{sourceID}" to the file state its fingerprint was
	// read from.
	Index map[string]FingerprintIndexEntry `json:"index"`

	// Entries maps a source.txt's MD5 to the fingerprint of its content.
	Entries map[string]FingerprintEntry `json:"entries"`
}

// NewFingerprintCache returns an empty cache for what this build computes.
func NewFingerprintCache() *FingerprintCache {
	return &FingerprintCache{
		SchemaVersion: fingerprintCacheSchemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Algo:          currentFingerprintAlgo(),
		Index:         map[string]FingerprintIndexEntry{},
		Entries:       map[string]FingerprintEntry{},
	}
}

// FingerprintKey is the index key of one source. Both halves are validated path
// segments, so neither can contain the separator or be empty.
func FingerprintKey(bookID, sourceID string) string {
	return bookID + "/" + sourceID
}

// Lookup returns the fingerprint recorded for key when stat still describes the
// file it was read from.
func (c *FingerprintCache) Lookup(key string, stat *FileStat) (FingerprintEntry, bool) {
	index, ok := c.Index[key]
	if !ok || !index.matches(stat) {
		return FingerprintEntry{}, false
	}

	// An index entry pointing at a fingerprint that is no longer stored is a
	// miss, not a hit with an empty answer. Reclamation can produce one on a
	// shelf several machines write to.
	entry, ok := c.Entries[index.MD5]
	return entry, ok
}

// point records that the file described by stat holds the content md5 names,
// without saying anything about the fingerprint of that content.
//
// The stat is the one taken BEFORE the content was read, never a fresh one. Read
// the other way round, a file rewritten while it was being read would be
// recorded with the newer state and the older content, and every later sweep
// would trust it. This order can only pair older state with newer content, which
// merely costs the next sweep one re-read.
func (c *FingerprintCache) point(key string, stat *FileStat, md5 string) {
	c.Index[key] = FingerprintIndexEntry{Size: stat.Size, ModTime: stat.ModTime, MD5: md5}
}

// record stores entry as the fingerprint of the content md5 names and points key
// at it. Storing never overwrites: entries are keyed by their own content, so a
// key that is already present already holds this answer.
func (c *FingerprintCache) record(key string, stat *FileStat, md5 string, entry FingerprintEntry) {
	c.point(key, stat, md5)
	if _, ok := c.Entries[md5]; !ok {
		c.Entries[md5] = entry
	}
}

// mergeFrom folds other into c.
//
// Entries are a union: they are keyed by their own content, so two records under
// one key are the same record and the worst a race can cost is a recomputation.
// An index key present in both keeps the newer modification time, which is the
// more recently observed state of that file. A tie goes to other, so a caller
// merging its own sweep last has its fresh observation kept.
func (c *FingerprintCache) mergeFrom(other *FingerprintCache) {
	for md5, entry := range other.Entries {
		if _, ok := c.Entries[md5]; !ok {
			c.Entries[md5] = entry
		}
	}

	for key, index := range other.Index {
		if existing, ok := c.Index[key]; ok && existing.ModTime.After(index.ModTime) {
			continue
		}
		c.Index[key] = index
	}
}

// prune drops what the shelf no longer refers to: index keys naming a book that
// is gone, and then fingerprints no remaining key points at.
//
// liveBookIDs comes from the shelf's own listing, so a book that was merely
// unreadable during one sweep still counts as live. Pruning too eagerly would
// only cost a recomputation, but recomputing a whole shelf is exactly what this
// file exists to prevent.
func (c *FingerprintCache) prune(liveBookIDs map[string]struct{}) {
	for key := range c.Index {
		bookID, _, ok := strings.Cut(key, "/")
		if !ok {
			delete(c.Index, key)
			continue
		}
		if _, live := liveBookIDs[bookID]; !live {
			delete(c.Index, key)
		}
	}

	referenced := make(map[string]struct{}, len(c.Index))
	for _, index := range c.Index {
		referenced[index.MD5] = struct{}{}
	}
	for md5 := range c.Entries {
		if _, ok := referenced[md5]; !ok {
			delete(c.Entries, md5)
		}
	}
}

// digest fingerprints the cache's content, so an unchanged sweep does not
// rewrite the file. The generator is left out deliberately, like the exported
// book cache leaves out its timestamp: upgrading the build is not a reason to
// rewrite megabytes onto an SMB share.
func (c *FingerprintCache) digest() (string, error) {
	data, err := json.Marshal(struct {
		Algo    FingerprintAlgo                  `json:"algo"`
		Index   map[string]FingerprintIndexEntry `json:"index"`
		Entries map[string]FingerprintEntry      `json:"entries"`
	}{Algo: c.Algo, Index: c.Index, Entries: c.Entries})
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// FingerprintOutcome says what one source cost.
type FingerprintOutcome int

const (
	// FingerprintReused answered from the index. The file was stat'ed and not
	// opened, which is the whole point of the cache.
	FingerprintReused FingerprintOutcome = iota

	// FingerprintDeduped had to read the file because its recorded state no
	// longer matched, but the content was already fingerprinted under some other
	// key - a copied or restored book. Only the index was updated.
	FingerprintDeduped

	// FingerprintComputed read the file and built a new fingerprint from it.
	FingerprintComputed
)

// FingerprintTarget names one source for the sweep to fingerprint.
//
// It is a plain value rather than a *Source on purpose: a sweep collects every
// target before it starts so it can report an honest total, and a Source holds
// its whole meta.json - including the split boundaries of a large book - which
// would then all be held at once for nothing.
type FingerprintTarget struct {
	BookID   string
	SourceID string

	// SourcePath is the source's folder relative to the shelf root.
	SourcePath string
}

// FingerprintTargets lists every source of every book, in listing order.
//
// It walks all sources rather than only each book's current one: a user who
// keeps an earlier import alongside a corrected one has two texts on the shelf,
// and a similarity report that cannot see one of them is misleading.
//
// A book whose sources cannot be listed at all is reported by ID in failed
// rather than aborting the listing, so one damaged package does not cost the
// whole sweep. A book that simply holds no source yet is not one of those: its
// sources/ directory is created with its first source, so a missing directory
// means an empty book and contributes nothing either way.
func (s *Shelf) FingerprintTargets() ([]FingerprintTarget, []string, error) {
	books, err := s.ListBooks()
	if err != nil {
		return nil, nil, util.Errorf("%w", err)
	}

	targets := make([]FingerprintTarget, 0, len(books))
	var failed []string

	for _, book := range books {
		sources, err := book.ListSource()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			failed = append(failed, book.ID())
			continue
		}
		for _, source := range sources {
			targets = append(targets, FingerprintTarget{
				BookID:     book.ID(),
				SourceID:   source.ID(),
				SourcePath: source.FolderPath(),
			})
		}
	}

	return targets, failed, nil
}

// Fingerprint resolves one target's fingerprint into cache, reading its content
// only when nothing already stored can answer.
//
// When the file has to be read it is read once and everything is derived from
// that single buffer - the raw MD5, the normalized text, its own hash and
// length, and the sketch. Source.refreshContentMetadata takes the same care for
// the same reason: on a network shelf each extra pass over a file is another
// round trip.
func (s *Shelf) Fingerprint(cache *FingerprintCache, target FingerprintTarget) (FingerprintOutcome, error) {
	key := FingerprintKey(target.BookID, target.SourceID)
	sourcePath := path.Join(target.SourcePath, SourceFile)

	stat, err := getFileStat(s.dbRoot, sourcePath)
	if err != nil {
		return FingerprintReused, util.Errorf("%w", err)
	}

	if _, ok := cache.Lookup(key, stat); ok {
		return FingerprintReused, nil
	}

	file, err := s.dbRoot.Open(sourcePath)
	if err != nil {
		return FingerprintReused, util.Errorf("%w", err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return FingerprintReused, util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return FingerprintReused, util.Errorf("%w", closeErr)
	}

	md5, err := hashutil.MD5Hash(bytes.NewReader(data))
	if err != nil {
		return FingerprintReused, util.Errorf("%w", err)
	}

	// This content may already be fingerprinted under another key: a book copied
	// into the shelf, restored from a backup, or simply re-imported. Only the
	// index needs to learn where it now also lives.
	if _, known := cache.Entries[md5]; known {
		cache.point(key, stat, md5)
		return FingerprintDeduped, nil
	}

	entry, err := buildFingerprint(data)
	if err != nil {
		return FingerprintReused, util.Errorf("%w", err)
	}

	cache.record(key, stat, md5, entry)
	return FingerprintComputed, nil
}

// buildFingerprint reduces one source's bytes to what a comparison needs.
func buildFingerprint(data []byte) (FingerprintEntry, error) {
	normalized := textnorm.Normalize(string(data))

	normMD5, err := hashutil.MD5Hash(strings.NewReader(normalized))
	if err != nil {
		return FingerprintEntry{}, util.Errorf("%w", err)
	}

	fingerprint := sketch.BuildDefault(normalized)

	return FingerprintEntry{
		NormMD5:   normMD5,
		NormChars: utf8.RuneCountInString(normalized),
		Shingles:  fingerprint.Distinct,
		Sketch:    fingerprint.Encode(),
	}, nil
}

// LoadFingerprintCache reads app/fingerprint-cache.json, returning an empty
// cache when there is nothing this build can use.
//
// Every failure is a cache miss and never an error: the file is absent on a
// fresh shelf, may have been written by a build that is not this one, and is
// rebuilt by the sweep that is about to run anyway.
func (s *Shelf) LoadFingerprintCache() *FingerprintCache {
	cache := s.readFingerprintCacheFile()
	if cache == nil {
		return NewFingerprintCache()
	}
	return cache
}

func (s *Shelf) readFingerprintCacheFile() *FingerprintCache {
	filePath := path.Join(appFolder, FingerprintCacheFileName)

	file, err := s.dbRoot.Open(filePath)
	if err != nil {
		s.Debug("no fingerprint cache to load", "path", filePath, "error", err)
		return nil
	}
	defer file.Close()

	var cache FingerprintCache
	if err := json.NewDecoder(file).Decode(&cache); err != nil {
		s.Debug("ignoring an unreadable fingerprint cache", "path", filePath, "error", err)
		return nil
	}
	if cache.SchemaVersion != fingerprintCacheSchemaVersion {
		s.Debug("ignoring a fingerprint cache this build does not read", "path", filePath, "schema_version", cache.SchemaVersion)
		return nil
	}
	if cache.Algo != currentFingerprintAlgo() {
		s.Debug("discarding a fingerprint cache built by another algorithm", "path", filePath, "algo", cache.Algo)
		return nil
	}

	if cache.Index == nil {
		cache.Index = map[string]FingerprintIndexEntry{}
	}
	if cache.Entries == nil {
		cache.Entries = map[string]FingerprintEntry{}
	}
	return &cache
}

// SaveFingerprintCache merges what a sweep learned into whatever is on disk now,
// reclaims what the shelf no longer refers to, and writes the result atomically.
//
// It re-reads the file rather than writing over the copy the sweep started from:
// a shelf is routinely open on more than one machine, and a sweep that takes
// half a minute is long enough for another one to finish in the meantime.
//
// A read-only shelf reports fsutil.ErrReadOnly. That is a real error here and
// not a shrug, unlike the scan cache: the caller asked for this work explicitly
// and would otherwise be told a sweep succeeded that kept nothing.
func (s *Shelf) SaveFingerprintCache(cache *FingerprintCache) error {
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	books, err := s.ListBooks()
	if err != nil {
		return util.Errorf("%w", err)
	}
	liveBookIDs := make(map[string]struct{}, len(books))
	for _, book := range books {
		liveBookIDs[book.ID()] = struct{}{}
	}

	merged := NewFingerprintCache()
	onDisk := s.readFingerprintCacheFile()
	if onDisk != nil {
		merged.mergeFrom(onDisk)
	}
	merged.mergeFrom(cache)
	merged.prune(liveBookIDs)

	digest, err := merged.digest()
	if err != nil {
		return util.Errorf("%w", err)
	}
	if onDisk != nil {
		previous, err := onDisk.digest()
		if err != nil {
			return util.Errorf("%w", err)
		}
		if previous == digest {
			s.Debug("fingerprint cache is unchanged", "sources", len(merged.Index), "fingerprints", len(merged.Entries))
			return nil
		}
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Atomic, like every other file under app/: a sync client copying the shelf
	// away must never pick up half a cache.
	filePath := path.Join(appFolder, FingerprintCacheFileName)
	if err := fsutil.WriteFileAtomic(root, filePath, data); err != nil {
		return util.Errorf("%w", err)
	}

	s.Debug("wrote the fingerprint cache", "path", filePath, "sources", len(merged.Index), "fingerprints", len(merged.Entries))
	return nil
}
