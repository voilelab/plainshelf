package repocheck

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// jsonV1Allowlist names every Go file still importing "encoding/json", mapped
// to the ticket that converts it. Anything not listed must use
// "encoding/json/v2" through internal/jsonopt.
//
// The list only shrinks. It exists because PSW-95 converts 96 call sites in
// four batches rather than one unreviewable PR, and the batches are worth
// nothing if a new file quietly reintroduces v1 alongside them: v1 and v2 do
// not write the same bytes, so a mixed repository has no single answer to
// "what does PlainShelf put in this file?". PSW-100 empties the map, at which
// point this test is what stops the import from coming back.
//
// Paths are slash-separated and relative to the repository root, and cover all
// three modules.
var jsonV1Allowlist = map[string]string{
	// PSW-97 — The shelf and its on-disk formats. Compatibility-sensitive: each
	// package re-blesses its own fixtures.
	"shelf/bookpkg/book.go":             "PSW-97",
	"shelf/bookpkg/book_test.go":        "PSW-97",
	"shelf/bookpkg/source.go":           "PSW-97",
	"shelf/bookpkg/source_test.go":      "PSW-97",
	"shelf/conformance_test.go":         "PSW-97",
	"shelf/fingerprint/cache.go":        "PSW-97",
	"shelf/fingerprint/cache_test.go":   "PSW-97",
	"shelf/fingerprint/helpers_test.go": "PSW-97",
	"shelf/scancache/cache.go":          "PSW-97",
	"shelf/shelf_cache_export.go":       "PSW-97",
	"shelf/shelf_cache_export_test.go":  "PSW-97",
	"shelf/shelf_config.go":             "PSW-97",
	"shelf/shelf_rescan_test.go":        "PSW-97",
	"shelf/shelf_test.go":               "PSW-97",
	"shelf/trash.go":                    "PSW-97",
	"shelf/trash_test.go":               "PSW-97",

	// PSW-98 — The HTTP surface of the server and the standalone reader.
	"reader/app_test.go":                                  "PSW-98",
	"reader/readerapi/api.go":                             "PSW-98",
	"reader/readerapi/api_test.go":                        "PSW-98",
	"reader/readerapi/spa.go":                             "PSW-98",
	"server/apicore.go":                                   "PSW-98",
	"server/apierr_test.go":                               "PSW-98",
	"server/contract/api_book_batch_contract_test.go":     "PSW-98",
	"server/contract/api_book_cache_contract_test.go":     "PSW-98",
	"server/contract/api_books_contract_test.go":          "PSW-98",
	"server/contract/api_content_stats_contract_test.go":  "PSW-98",
	"server/contract/api_scans_contract_test.go":          "PSW-98",
	"server/contract/api_schema_version_contract_test.go": "PSW-98",
	"server/contract/apitest_assert_test.go":              "PSW-98",
	"server/contract/apitest_book_test.go":                "PSW-98",
	"server/contract/apitest_taskchain_test.go":           "PSW-98",
	"server/handle_books.go":                              "PSW-98",
	"server/handle_fingerprints_test.go":                  "PSW-98",
	"server/handle_folders.go":                            "PSW-98",
	"server/import_epub.go":                               "PSW-98",
	"server/settings.go":                                  "PSW-98",
	"server/spa.go":                                       "PSW-98",

	// PSW-100 — Everything the batches above do not reach.
	"desktop/app.go":                       "PSW-100",
	"desktop/shelves.go":                   "PSW-100",
	"internal/readingprogress/document.go": "PSW-100",
	"internal/util/json_date_test.go":      "PSW-100",
	"internal/util/json_time_test.go":      "PSW-100",
}

// TestNoEncodingJSONV1Imports fails on any Go file importing "encoding/json"
// that the allowlist does not excuse.
func TestNoEncodingJSONV1Imports(t *testing.T) {
	root := repoRoot(t)

	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, pth)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, pth, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Imports {
			pathValue, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil || pathValue != "encoding/json" {
				continue
			}
			if ticket, ok := jsonV1Allowlist[rel]; ok {
				if ticket == "" {
					t.Errorf("%s: allowlisted without the ticket that converts it", rel)
				}
				continue
			}
			t.Errorf("%s:%d: imports \"encoding/json\"; use \"encoding/json/v2\" with the options from internal/jsonopt, or add the file to jsonV1Allowlist with the ticket that converts it",
				rel, fset.Position(spec.Pos()).Line)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk the repository: %v", err)
	}
}

// TestJSONV1AllowlistIsCurrent keeps the allowlist honest in both directions: a
// converted or deleted file must lose its entry, so the list is a live count of
// the work left rather than a permanent exemption for a path.
func TestJSONV1AllowlistIsCurrent(t *testing.T) {
	root := repoRoot(t)

	var stale []string
	for rel := range jsonV1Allowlist {
		pth := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(pth); err != nil {
			t.Errorf("Allowlisted file %s is gone; drop the entry: %v", rel, err)
			continue
		}
		if !importsJSONV1(t, pth) {
			stale = append(stale, rel)
		}
	}

	sort.Strings(stale)
	for _, rel := range stale {
		t.Errorf("%s no longer imports \"encoding/json\"; drop its jsonV1Allowlist entry", rel)
	}
}

func importsJSONV1(t *testing.T, pth string) bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), pth, nil, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("Failed to parse %s: %v", pth, err)
	}
	for _, spec := range file.Imports {
		if value, err := strconv.Unquote(spec.Path.Value); err == nil && value == "encoding/json" {
			return true
		}
	}
	return false
}
