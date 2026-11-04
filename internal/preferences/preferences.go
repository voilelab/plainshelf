package preferences

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/lockedfile"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
)

const documentVersion = 1

const (
	AppDesktop = "desktop"
	AppReader  = "reader"
)

const (
	maxKeyBytes   = 128
	maxValueBytes = 4096
)

var appScopedKeys = []string{"plainshelf.shelf"}

func isAppScoped(key string) bool {
	return slices.Contains(appScopedKeys, key)
}

type Document struct {
	Version int                          `json:"version"`
	Shared  map[string]string            `json:"shared"`
	Apps    map[string]map[string]string `json:"apps"`
}

func New() Document {
	return Document{Version: documentVersion, Shared: map[string]string{}, Apps: map[string]map[string]string{}}
}

func Parse(text string) Document {
	var raw struct {
		Version float64                   `json:"version"`
		Shared  jsontext.Value            `json:"shared"`
		Apps    map[string]jsontext.Value `json:"apps"`
	}
	if strings.TrimSpace(text) == "" || json.Unmarshal([]byte(text), &raw) != nil {
		return New()
	}
	if raw.Version != documentVersion {
		return New()
	}

	doc := New()
	doc.Shared = parseValues(raw.Shared)
	for app, value := range raw.Apps {
		if values := parseValues(value); len(values) > 0 {
			doc.Apps[app] = values
		}
	}
	return doc
}

func parseValues(value jsontext.Value) map[string]string {
	out := map[string]string{}
	var entries map[string]jsontext.Value
	if json.Unmarshal(value, &entries) != nil {
		return out
	}
	for key, entry := range entries {
		var text string
		if validKey(key) == nil && json.Unmarshal(entry, &text) == nil && text != "" {
			out[key] = text
		}
	}
	return out
}

func Serialize(doc Document) (string, error) {
	if doc.Shared == nil {
		doc.Shared = map[string]string{}
	}
	if doc.Apps == nil {
		doc.Apps = map[string]map[string]string{}
	}
	bs, err := json.Marshal(doc, jsonopt.Disk())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

func (d Document) For(app string) map[string]string {
	out := maps.Clone(d.Shared)
	for key, value := range d.Apps[app] {
		if isAppScoped(key) {
			out[key] = value
		}
	}
	return out
}

func withValue(doc Document, app, key, value string) Document {
	out := Document{Version: doc.Version, Shared: maps.Clone(doc.Shared), Apps: map[string]map[string]string{}}
	for name, values := range doc.Apps {
		out.Apps[name] = maps.Clone(values)
	}

	target := out.Shared
	if isAppScoped(key) {
		if out.Apps[app] == nil {
			out.Apps[app] = map[string]string{}
		}
		target = out.Apps[app]
	}
	if value == "" {
		delete(target, key)
	} else {
		target[key] = value
	}
	if len(out.Apps[app]) == 0 {
		delete(out.Apps, app)
	}
	return out
}

func validKey(key string) error {
	if key == "" || len(key) > maxKeyBytes || strings.TrimSpace(key) != key {
		return util.Errorf("invalid preference key %q", key)
	}
	return nil
}

type Store struct {
	file *lockedfile.File
	app  string
}

func NewStore(path, app string) *Store {
	return &Store{file: lockedfile.New(path, "preferences", ".preferences-*.json"), app: app}
}

func (s *Store) Read() (map[string]string, error) {
	text, err := s.file.Read()
	if err != nil {
		return nil, err
	}
	return Parse(text).For(s.app), nil
}

func (s *Store) Set(key, value string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if len(value) > maxValueBytes {
		return util.Errorf("preference %q is too large: %d bytes", key, len(value))
	}
	_, err := lockedfile.Mutate(s.file, Parse, Serialize, func(doc Document) Document {
		return withValue(doc, s.app, key, value)
	})
	return err
}

func DefaultPath() (string, error) {
	dir, err := readingprogress.SharedDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "preferences.json"), nil
}
