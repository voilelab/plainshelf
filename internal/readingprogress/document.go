package readingprogress

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"maps"
	"math"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

const DocumentVersion = 2

const ReaderShelfID = "book"

// maxSafeInteger mirrors JavaScript's Number.MAX_SAFE_INTEGER: the frontend
// stores offsets and timestamps as JSON numbers, so anything larger could not
// round-trip.
const maxSafeInteger = 1<<53 - 1

type Entry struct {
	Offset int64 `json:"offset"`
	At     int64 `json:"at"`
}

type Document struct {
	Version int                         `json:"version"`
	Shelves map[string]map[string]Entry `json:"shelves"`
}

func New() Document {
	return Document{Version: DocumentVersion, Shelves: map[string]map[string]Entry{}}
}

func Parse(text string) Document {
	doc := New()

	if strings.TrimSpace(text) == "" {
		return doc
	}

	var raw struct {
		Version int `json:"version"`
		Shelves map[string]map[string]struct {
			Offset float64 `json:"offset"`
			At     float64 `json:"at"`
		} `json:"shelves"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return doc
	}
	if raw.Version != DocumentVersion {
		return doc
	}

	for shelfKey, books := range raw.Shelves {
		trimmedKey := strings.TrimSpace(shelfKey)
		if trimmedKey == "" {
			continue
		}
		normalized := map[string]Entry{}
		for bookID, entry := range books {
			trimmedID := strings.TrimSpace(bookID)
			value, ok := normalizeEntry(entry.Offset, entry.At)
			if trimmedID != "" && ok {
				normalized[trimmedID] = value
			}
		}
		if len(normalized) > 0 {
			doc.Shelves[trimmedKey] = normalized
		}
	}

	return doc
}

func ParseStrict(text string) (Document, error) {
	if strings.TrimSpace(text) != "" && !jsontext.Value(text).IsValid() {
		return New(), util.NewError("reading progress document is not valid JSON")
	}
	return Parse(text), nil
}

func Serialize(doc Document) (string, error) {
	if doc.Version == 0 {
		doc.Version = DocumentVersion
	}
	if doc.Shelves == nil {
		doc.Shelves = map[string]map[string]Entry{}
	}
	bs, err := json.Marshal(doc, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

func (d Document) Clone() Document {
	out := Document{Version: d.Version, Shelves: make(map[string]map[string]Entry, len(d.Shelves))}
	if out.Version == 0 {
		out.Version = DocumentVersion
	}
	for shelfKey, books := range d.Shelves {
		out.Shelves[shelfKey] = maps.Clone(books)
	}
	return out
}

type ResolveShelf func(bookID string) (realShelfID string, ok bool)

func Project(doc Document, readerShelfID string, resolve ResolveShelf) Document {
	out := doc.Clone()
	readerShelfID = strings.TrimSpace(readerShelfID)
	readerBooks := out.Shelves[readerShelfID]
	if readerShelfID == "" || len(readerBooks) == 0 || resolve == nil {
		return out
	}

	for bookID, entry := range readerBooks {
		realShelfID, ok := resolve(bookID)
		if !ok {
			continue
		}
		realShelfID = strings.TrimSpace(realShelfID)
		if realShelfID == "" || realShelfID == readerShelfID {
			continue
		}
		dst := out.Shelves[realShelfID]
		if dst == nil {
			dst = map[string]Entry{}
			out.Shelves[realShelfID] = dst
		}
		if cur, exists := dst[bookID]; !exists || entry.At >= cur.At {
			dst[bookID] = entry
		}
	}

	return out
}

func MergeNewest(disk, write Document) Document {
	out := disk.Clone()
	for shelfKey, books := range write.Shelves {
		shelfKey = strings.TrimSpace(shelfKey)
		if shelfKey == "" || len(books) == 0 {
			continue
		}
		dst := out.Shelves[shelfKey]
		if dst == nil {
			dst = map[string]Entry{}
			out.Shelves[shelfKey] = dst
		}
		for bookID, entry := range books {
			bookID = strings.TrimSpace(bookID)
			if bookID == "" {
				continue
			}
			if cur, exists := dst[bookID]; !exists || entry.At >= cur.At {
				dst[bookID] = entry
			}
		}
	}
	return out
}

func normalizeEntry(offset, at float64) (Entry, bool) {
	normalizedOffset, ok := normalizeNumber(offset)
	if !ok {
		return Entry{}, false
	}
	normalizedAt, ok := normalizeNumber(at)
	if !ok {
		normalizedAt = 0
	}
	if normalizedOffset == 0 && normalizedAt == 0 {
		return Entry{}, false
	}
	return Entry{Offset: normalizedOffset, At: normalizedAt}, true
}

func normalizeNumber(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value < 0 || value > maxSafeInteger {
		return 0, false
	}
	if value != math.Trunc(value) {
		return 0, false
	}
	return int64(value), true
}
