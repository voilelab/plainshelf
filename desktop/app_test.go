package main

import (
	"net/http/httptest"
	"testing"
)

func TestBookOpenDialogOptions(t *testing.T) {
	options := bookOpenDialogOptions()
	if len(options.Filters) != 1 {
		t.Fatalf("expected exactly one file filter, got %d", len(options.Filters))
	}

	filter := options.Filters[0]
	if filter.Pattern != "*.txt" {
		t.Fatalf("expected txt-only filter pattern, got %q", filter.Pattern)
	}
}

func TestNormalizeSelectedLocalPaths(t *testing.T) {
	paths := normalizeSelectedLocalPaths([]string{"", "  ", " /tmp/book-1.txt ", "/tmp/book-2.txt"})
	if len(paths) != 2 {
		t.Fatalf("expected two valid paths, got %d", len(paths))
	}
	if paths[0] != "/tmp/book-1.txt" {
		t.Fatalf("unexpected first path: %q", paths[0])
	}
	if paths[1] != "/tmp/book-2.txt" {
		t.Fatalf("unexpected second path: %q", paths[1])
	}
}

func TestNormalizeLayerParts(t *testing.T) {
	parts := normalizeLayerParts([]string{"", "  ", " fiction ", " sci-fi "})
	if len(parts) != 2 {
		t.Fatalf("expected two valid layer parts, got %d", len(parts))
	}
	if parts[0] != "fiction" {
		t.Fatalf("unexpected first part: %q", parts[0])
	}
	if parts[1] != "sci-fi" {
		t.Fatalf("unexpected second part: %q", parts[1])
	}
}

func TestIsAPIPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "standard root", path: "/api", want: true},
		{name: "standard nested", path: "/api/books/1", want: true},
		{name: "wails prefixed path", path: "/wails/api/books/1", want: true},
		{name: "missing leading slash", path: "api/books/1", want: false},
		{name: "missing leading slash wails prefix", path: "wails/api/books/1", want: false},
		{name: "frontend route", path: "/books/1/sources", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAPIPath(tc.path); got != tc.want {
				t.Fatalf("isAPIPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestNormalizeAPIRequest(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "standard api path unchanged", path: "/api/books/1", want: "/api/books/1"},
		{name: "wails api path normalized", path: "/wails/api/books/1", want: "/api/books/1"},
		{name: "wails api root normalized", path: "/wails/api", want: "/api"},
		{name: "non api path unchanged", path: "/books/1", want: "/books/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			got := normalizeAPIRequest(req)
			if got.URL.Path != tt.want {
				t.Fatalf("normalizeAPIRequest(%q) path = %q, want %q", tt.path, got.URL.Path, tt.want)
			}
		})
	}
}
