package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func TestParseDefaultSplit(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    shelf.SplitConfig
		wantErr string
	}{
		{
			name: "empty means no default",
			raw:  "",
			want: shelf.SplitConfig{},
		},
		{
			name: "blank means no default",
			raw:  "   ",
			want: shelf.SplitConfig{},
		},
		{
			name: "line count",
			raw:  `{"type":"line_count","line_count":500}`,
			want: shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 500},
		},
		{
			name: "regex",
			raw:  `{"type":"regex","regex":"^Chapter \\d+$"}`,
			want: shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^Chapter \d+$`},
		},
		{
			name: "an explicit none is accepted",
			raw:  `{"type":""}`,
			want: shelf.SplitConfig{},
		},
		{
			name:    "malformed JSON is refused",
			raw:     `{"type":`,
			wantErr: "parse -default-split-config",
		},
		{
			name:    "an unknown type is refused rather than silently ignored",
			raw:     `{"type":"paragraph"}`,
			wantErr: `unknown split type "paragraph"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDefaultSplit(test.raw)

			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("parseDefaultSplit(%q) = %+v, want an error", test.raw, got)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("error = %q, want it to mention %q", err, test.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseDefaultSplit(%q) returned error: %v", test.raw, err)
			}
			if got.Type != test.want.Type || got.LineCount != test.want.LineCount || got.Regex != test.want.Regex {
				t.Errorf("parseDefaultSplit(%q) = %+v, want %+v", test.raw, got, test.want)
			}
		})
	}
}

// TestCheckShelfDir covers the guard that keeps a mistyped path from being
// created as an empty shelf and reported as a clean run.
func TestCheckShelfDir(t *testing.T) {
	shelfPath := t.TempDir()
	if err := checkShelfDir(shelfPath); err == nil {
		t.Errorf("checkShelfDir accepted a directory with no %s/", booksFolder)
	}

	if err := os.Mkdir(filepath.Join(shelfPath, booksFolder), 0o755); err != nil {
		t.Fatalf("create books dir: %v", err)
	}
	if err := checkShelfDir(shelfPath); err != nil {
		t.Errorf("checkShelfDir rejected a real shelf: %v", err)
	}

	if err := checkShelfDir(filepath.Join(shelfPath, "nope")); err == nil {
		t.Errorf("checkShelfDir accepted a path that does not exist")
	}

	// A file named books/ is not a shelf either.
	filePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(filePath, booksFolder), []byte("x"), 0o644); err != nil {
		t.Fatalf("write books file: %v", err)
	}
	if err := checkShelfDir(filePath); err == nil {
		t.Errorf("checkShelfDir accepted a shelf whose books/ is a file")
	}
}

func TestSplitBookIDs(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , ,b ", []string{"a", "b"}},
	}

	for _, test := range tests {
		got := splitBookIDs(test.raw)
		if len(got) != len(test.want) {
			t.Errorf("splitBookIDs(%q) = %v, want %v", test.raw, got, test.want)
			continue
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("splitBookIDs(%q) = %v, want %v", test.raw, got, test.want)
				break
			}
		}
	}
}

func TestDescribeSplit(t *testing.T) {
	tests := []struct {
		config shelf.SplitConfig
		want   string
	}{
		{shelf.SplitConfig{}, "none"},
		{shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 20}, "line_count(20)"},
		{shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: "^x$"}, `regex("^x$")`},
		{shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{1, 2}}, "boundary(2 lines)"},
	}

	for _, test := range tests {
		if got := describeSplit(test.config); got != test.want {
			t.Errorf("describeSplit(%+v) = %q, want %q", test.config, got, test.want)
		}
	}
}
