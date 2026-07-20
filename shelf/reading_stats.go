package shelf

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
Reading time tracking is stored under {LibRoot}/app/stats/reading/YYYY-MM.json,
one file per calendar month, e.g.:

	{
	  "version": 1,
	  "month": "2026-07",
	  "days": {
	    "2026-07-13": {"total_seconds": 4200, "books": {"book-id": 3000}}
	  }
	}

Months are loaded lazily on first access and kept in memory. Writes only ever
touch the in-memory map; a background flush (owned by Shelf) periodically
persists dirty months to disk so heartbeats never trigger a disk write directly
(important for SMB-mounted libraries).
*/

const statsFolder = "stats"
const statsReadingFolder = "reading"
const readingStatsVersion = 1

// Per-request/day clamps. These keep a single misbehaving client (or clock skew)
// from corrupting stats with unbounded values.
const maxSecondsPerCall = 120
const maxSecondsPerDay = 24 * 60 * 60 // 86400

const dateLayout = "2006-01-02"
const monthLayout = "2006-01"

type readingDayStats struct {
	TotalSeconds int            `json:"total_seconds"`
	Books        map[string]int `json:"books,omitempty"`
}

type readingMonthFile struct {
	Version int                         `json:"version"`
	Month   string                      `json:"month"`
	Days    map[string]*readingDayStats `json:"days"`
}

// ReadingStats tracks per-day, per-book reading time for a shelf.
type ReadingStats struct {
	root fsutil.FS

	mu     sync.Mutex
	months map[string]*readingMonthFile
	dirty  map[string]struct{}
	// gens counts mutations per month (never reset). Flush snapshots it so it
	// only clears a month's dirty mark if no AddSeconds landed while the
	// snapshot was being written to disk outside the lock.
	gens map[string]uint64
}

func newReadingStats(root fsutil.FS) *ReadingStats {
	return &ReadingStats{
		root:   root,
		months: make(map[string]*readingMonthFile),
		dirty:  make(map[string]struct{}),
		gens:   make(map[string]uint64),
	}
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func readingStatsFilePath(monthKey string) string {
	return path.Join(appFolder, statsFolder, statsReadingFolder, monthKey+".json")
}

// loadMonthLocked returns the in-memory month data, lazily loading it from disk
// if not already present. Caller must hold r.mu.
func (r *ReadingStats) loadMonthLocked(monthKey string) (*readingMonthFile, error) {
	if data, ok := r.months[monthKey]; ok {
		return data, nil
	}

	data := &readingMonthFile{
		Version: readingStatsVersion,
		Month:   monthKey,
		Days:    make(map[string]*readingDayStats),
	}

	f, err := r.root.Open(readingStatsFilePath(monthKey))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No stats recorded for this month yet (or a pre-existing vault
			// created before reading stats existed). Treat as empty - no
			// migration needed.
			r.months[monthKey] = data
			return data, nil
		}
		return nil, util.Errorf("%w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	if err := decoder.Decode(data); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if data.Days == nil {
		data.Days = make(map[string]*readingDayStats)
	}

	r.months[monthKey] = data
	return data, nil
}

// AddSeconds records additional reading time for a book on a given date.
// sec is clamped to [0, maxSecondsPerCall]; the cumulative per-day total (both
// overall and per-book) is clamped to maxSecondsPerDay. Only in-memory state is
// touched - callers rely on the periodic/close-time Flush to persist.
func (r *ReadingStats) AddSeconds(date time.Time, bookID string, sec int) error {
	if bookID == "" {
		return util.NewError("book id cannot be empty")
	}

	sec = clampInt(sec, 0, maxSecondsPerCall)
	monthKey := date.Format(monthLayout)
	dateKey := date.Format(dateLayout)

	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := r.loadMonthLocked(monthKey)
	if err != nil {
		return util.Errorf("%w", err)
	}

	day, ok := data.Days[dateKey]
	if !ok {
		day = &readingDayStats{Books: make(map[string]int)}
		data.Days[dateKey] = day
	}
	if day.Books == nil {
		day.Books = make(map[string]int)
	}

	day.Books[bookID] = clampInt(day.Books[bookID]+sec, 0, maxSecondsPerDay)
	day.TotalSeconds = clampInt(day.TotalSeconds+sec, 0, maxSecondsPerDay)

	r.dirty[monthKey] = struct{}{}
	r.gens[monthKey]++

	return nil
}

// DayTotal is the aggregate reading time for a single day, as exposed to API callers.
type DayTotal struct {
	TotalSeconds int
}

// Range returns the per-day total reading seconds for dates within [from, to]
// (inclusive). Only days that actually have recorded data are included in the
// result - a shelf/vault with no reading stats yet (or months with no file on
// disk) simply contributes nothing, so this never errors on missing files.
func (r *ReadingStats) Range(from, to time.Time) (map[string]DayTotal, error) {
	if to.Before(from) {
		return nil, util.NewError("to date must not be before from date")
	}

	// Collect the set of months spanned by [from, to].
	months := make([]string, 0)
	seen := make(map[string]struct{})
	for m := from; !m.After(to); m = m.AddDate(0, 0, 1) {
		key := m.Format(monthLayout)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			months = append(months, key)
		}
	}

	result := make(map[string]DayTotal)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, month := range months {
		data, err := r.loadMonthLocked(month)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		for date, day := range data.Days {
			dt, err := time.Parse(dateLayout, date)
			if err != nil {
				continue
			}
			if dt.Before(from) || dt.After(to) {
				continue
			}
			result[date] = DayTotal{TotalSeconds: day.TotalSeconds}
		}
	}

	return result, nil
}

// Flush persists any dirty months to disk using a tmp-file + rename so a crash
// mid-write never leaves a half-written stats file behind.
func (r *ReadingStats) Flush() error {
	r.mu.Lock()
	if len(r.dirty) == 0 {
		r.mu.Unlock()
		return nil
	}
	toFlush := make(map[string]readingMonthFile, len(r.dirty))
	for month := range r.dirty {
		data := r.months[month]
		if data == nil {
			continue
		}
		// Shallow copy is enough: Days map itself isn't replaced elsewhere while
		// holding the lock, and we only marshal it below (also under lock via
		// the outer scope of this snapshot). To be safe from concurrent
		// AddSeconds mutating the map while we marshal it after unlocking, we
		// marshal while still holding the lock instead of deferring to later.
		toFlush[month] = *data
	}
	dirtyMonths := make([]string, 0, len(r.dirty))
	snapGens := make(map[string]uint64, len(r.dirty))
	for month := range r.dirty {
		dirtyMonths = append(dirtyMonths, month)
		snapGens[month] = r.gens[month]
	}

	// Marshal while still holding the lock so concurrent AddSeconds calls can't
	// race with json.Marshal reading the Days map.
	marshaled := make(map[string][]byte, len(toFlush))
	for month, data := range toFlush {
		bs, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			r.mu.Unlock()
			return util.Errorf("%w", err)
		}
		marshaled[month] = append(bs, '\n')
	}
	r.mu.Unlock()

	if err := r.root.MkdirAll(path.Join(appFolder, statsFolder, statsReadingFolder)); err != nil {
		return util.Errorf("%w", err)
	}

	flushedMonths := make([]string, 0, len(dirtyMonths))
	var firstErr error
	for _, month := range dirtyMonths {
		bs, ok := marshaled[month]
		if !ok {
			continue
		}
		if err := r.writeMonthFile(month, bs); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		flushedMonths = append(flushedMonths, month)
	}

	// Only clear the months that actually made it to disk, and only if no
	// AddSeconds re-dirtied them while we were writing outside the lock — the
	// just-written snapshot doesn't contain those later heartbeats, so such
	// months must stay dirty for the next Flush. Failed months stay dirty too.
	r.mu.Lock()
	for _, month := range flushedMonths {
		if r.gens[month] == snapGens[month] {
			delete(r.dirty, month)
		}
	}
	r.mu.Unlock()

	if firstErr != nil {
		return util.Errorf("%w", firstErr)
	}

	return nil
}

func (r *ReadingStats) writeMonthFile(month string, bs []byte) error {
	filePath := readingStatsFilePath(month)
	tmpPath := filePath + ".tmp"

	if err := r.root.WriteFile(tmpPath, bs); err != nil {
		return util.Errorf("%w", err)
	}
	if err := r.root.Rename(tmpPath, filePath); err != nil {
		_ = r.root.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	return nil
}
