package readhistory

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/lockedfile"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
)

const documentVersion = 1

const DefaultLimit = 100

type Document struct {
	Version int                 `json:"version"`
	Limit   int                 `json:"limit"`
	Shelves map[string][]string `json:"shelves"`
}

func New() Document {
	return Document{Version: documentVersion, Limit: DefaultLimit, Shelves: map[string][]string{}}
}

func Parse(text string) Document {
	var raw struct {
		Version float64        `json:"version"`
		Limit   jsontext.Value `json:"limit"`
		Shelves jsontext.Value `json:"shelves"`
	}
	if strings.TrimSpace(text) == "" || json.Unmarshal([]byte(text), &raw) != nil {
		return New()
	}
	if raw.Version != documentVersion {
		return New()
	}

	doc := New()
	doc.Limit = parseLimit(raw.Limit)

	var shelves map[string]jsontext.Value
	if json.Unmarshal(raw.Shelves, &shelves) != nil {
		return doc
	}
	for shelfKey, value := range shelves {
		if ids := parseIDs(value, doc.Limit); len(ids) > 0 {
			doc.Shelves[shelfKey] = ids
		}
	}
	return doc
}

func parseLimit(value jsontext.Value) int {
	var limit *float64
	if json.Unmarshal(value, &limit) != nil || limit == nil {
		return DefaultLimit
	}
	return normalizeLimit(*limit)
}

func normalizeLimit(limit float64) int {
	if limit < 0 || limit != math.Trunc(limit) || math.IsInf(limit, 0) {
		return DefaultLimit
	}
	return int(min(limit, math.MaxInt32))
}

func parseIDs(value jsontext.Value, limit int) []string {
	var entries []jsontext.Value
	if limit <= 0 || json.Unmarshal(value, &entries) != nil {
		return nil
	}

	var ids []string
	seen := map[string]bool{}
	for _, entry := range entries {
		var id string
		if json.Unmarshal(entry, &id) != nil {
			continue
		}
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func Serialize(doc Document) (string, error) {
	if doc.Shelves == nil {
		doc.Shelves = map[string][]string{}
	}
	bs, err := json.Marshal(doc, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

func (d Document) clone() Document {
	out := Document{Version: d.Version, Limit: d.Limit, Shelves: make(map[string][]string, len(d.Shelves))}
	for shelfKey, ids := range d.Shelves {
		out.Shelves[shelfKey] = slices.Clone(ids)
	}
	return out
}

func withEntry(doc Document, shelfKey, bookID string) Document {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" || doc.Limit <= 0 {
		return doc
	}

	out := doc.clone()
	ids := []string{bookID}
	for _, id := range doc.Shelves[shelfKey] {
		if id != bookID {
			ids = append(ids, id)
		}
	}
	out.Shelves[shelfKey] = ids[:min(len(ids), doc.Limit)]
	return out
}

func withoutShelf(doc Document, shelfKey string) Document {
	out := doc.clone()
	delete(out.Shelves, shelfKey)
	return out
}

func withLimit(doc Document, limit int) Document {
	out := doc.clone()
	out.Limit = normalizeLimit(float64(limit))
	for shelfKey, ids := range out.Shelves {
		if out.Limit == 0 {
			delete(out.Shelves, shelfKey)
			continue
		}
		out.Shelves[shelfKey] = ids[:min(len(ids), out.Limit)]
	}
	return out
}

type Store struct {
	file *lockedfile.File
}

func NewStore(path string) *Store {
	return &Store{file: lockedfile.New(path, "read history", ".read_history-*.json")}
}

func (s *Store) Read() (string, error) {
	return s.file.Read()
}

func (s *Store) Record(shelfKey, bookID string) error {
	return s.mutate(func(doc Document) Document { return withEntry(doc, shelfKey, bookID) })
}

func (s *Store) Clear(shelfKey string) error {
	return s.mutate(func(doc Document) Document { return withoutShelf(doc, shelfKey) })
}

func (s *Store) SetLimit(limit int) error {
	return s.mutate(func(doc Document) Document { return withLimit(doc, limit) })
}

func (s *Store) mutate(fn func(Document) Document) error {
	_, err := lockedfile.Mutate(s.file, Parse, Serialize, fn)
	return err
}

func Project(doc Document, readerShelfID string, resolve func(bookID string) (string, bool)) Document {
	readerIDs := doc.Shelves[readerShelfID]
	if readerShelfID == "" || len(readerIDs) == 0 || resolve == nil {
		return doc
	}

	out := doc.clone()
	var kept []string
	moved := map[string][]string{}
	var targets []string
	for _, bookID := range readerIDs {
		target, ok := resolve(bookID)
		target = strings.TrimSpace(target)
		if !ok || target == "" || target == readerShelfID {
			kept = append(kept, bookID)
			continue
		}
		if _, seen := moved[target]; !seen {
			targets = append(targets, target)
		}
		moved[target] = append(moved[target], bookID)
	}
	if len(targets) == 0 {
		return doc
	}

	for _, target := range targets {
		ids := moved[target]
		for _, id := range doc.Shelves[target] {
			if !slices.Contains(moved[target], id) {
				ids = append(ids, id)
			}
		}
		out.Shelves[target] = ids[:min(len(ids), out.Limit)]
	}
	if len(kept) == 0 {
		delete(out.Shelves, readerShelfID)
	} else {
		out.Shelves[readerShelfID] = kept
	}
	return out
}

func (s *Store) ProjectReader(readerShelfID string, resolve func(bookID string) (string, bool)) (string, error) {
	raw, err := s.file.Read()
	if err != nil {
		return "", err
	}
	if len(Parse(raw).Shelves[readerShelfID]) == 0 {
		return raw, nil
	}
	doc, err := lockedfile.Mutate(s.file, Parse, Serialize, func(doc Document) Document {
		return Project(doc, readerShelfID, resolve)
	})
	if err != nil {
		return "", err
	}
	return Serialize(doc)
}

func DefaultPath() (string, error) {
	dir, err := readingprogress.SharedDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "read_history.json"), nil
}
