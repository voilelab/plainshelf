package readingstats

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/lockedfile"
	"github.com/voilelab/plainshelf/internal/util"
)

const documentVersion = 1

const (
	RetentionDays     = 400
	maxSecondsPerCall = 120
	maxSecondsPerDay  = 24 * 60 * 60
)

const dateLayout = time.DateOnly

type Document struct {
	Version int                         `json:"version"`
	Shelves map[string]map[string]int64 `json:"shelves"`
}

func New() Document {
	return Document{Version: documentVersion, Shelves: map[string]map[string]int64{}}
}

func Parse(text string) Document {
	var raw struct {
		Version float64                   `json:"version"`
		Shelves map[string]jsontext.Value `json:"shelves"`
	}
	if strings.TrimSpace(text) == "" || json.Unmarshal([]byte(text), &raw) != nil {
		return New()
	}
	if raw.Version != documentVersion {
		return New()
	}

	doc := New()
	for key, value := range raw.Shelves {
		if days := parseDays(value); len(days) > 0 {
			doc.Shelves[key] = days
		}
	}
	return doc
}

func parseDays(value jsontext.Value) map[string]int64 {
	var entries map[string]jsontext.Value
	if json.Unmarshal(value, &entries) != nil {
		return nil
	}

	days := map[string]int64{}
	for date, entry := range entries {
		var seconds float64
		if !isValidDate(date) || json.Unmarshal(entry, &seconds) != nil {
			continue
		}
		if normalized := clampSeconds(seconds, maxSecondsPerDay); normalized > 0 {
			days[date] = normalized
		}
	}
	return days
}

func isValidDate(date string) bool {
	parsed, err := time.Parse(dateLayout, date)
	return err == nil && parsed.Format(dateLayout) == date
}

func clampSeconds(seconds float64, limit int64) int64 {
	if math.IsNaN(seconds) || seconds <= 0 {
		return 0
	}
	return min(int64(math.Round(min(seconds, float64(limit)))), limit)
}

func Serialize(doc Document) (string, error) {
	if doc.Shelves == nil {
		doc.Shelves = map[string]map[string]int64{}
	}
	bs, err := json.Marshal(doc, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

func withSeconds(doc Document, key, date string, seconds float64, now time.Time) Document {
	added := clampSeconds(seconds, maxSecondsPerCall)
	today := now.Format(dateLayout)
	if added <= 0 || key == "" || !isValidDate(date) || date > today {
		return doc
	}

	year, month, day := now.Date()
	oldest := time.Date(year, month, day, 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(RetentionDays - 1)).
		Format(dateLayout)
	if date < oldest {
		return doc
	}

	days := map[string]int64{}
	for existing, value := range doc.Shelves[key] {
		if existing >= oldest && existing <= today {
			days[existing] = value
		}
	}
	days[date] = min(days[date]+added, maxSecondsPerDay)

	out := Document{Version: doc.Version, Shelves: maps.Clone(doc.Shelves)}
	if out.Shelves == nil {
		out.Shelves = map[string]map[string]int64{}
	}
	out.Shelves[key] = days
	return out
}

type Store struct {
	file *lockedfile.File
}

func NewStore(path string) *Store {
	return &Store{file: lockedfile.New(path, "reading stats", ".reading_stats-*.json")}
}

func (s *Store) Read() (string, error) {
	return s.file.Read()
}

func (s *Store) Add(key, date string, seconds float64, now time.Time) error {
	_, err := lockedfile.Mutate(s.file, Parse, Serialize, func(doc Document) Document {
		return withSeconds(doc, key, date, seconds, now)
	})
	return err
}
