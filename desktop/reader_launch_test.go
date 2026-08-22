package main

import (
	"reflect"
	"testing"
)

func TestReaderLaunchCommandDefault(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	name, args := readerLaunchCommand("/books/alpha.bookpkg", "shelf-real")
	if name != "open" {
		t.Fatalf("launcher = %q, want open", name)
	}
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-shelf", "shelf-real"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestReaderLaunchCommandEnvOverride(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "/tmp/build/PlainShelfReader.app")

	_, args := readerLaunchCommand("/books/beta.bookpkg", "shelf-real")
	want := []string{"-n", "-a", "/tmp/build/PlainShelfReader.app", "--args", "-book", "/books/beta.bookpkg", "-shelf", "shelf-real"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

// A launch with no real shelf id (empty -shelf) must omit the flag entirely, so
// the reader falls back to its synthetic shelf rather than reporting an empty
// shelf as active.
func TestReaderLaunchCommandNoShelf(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	_, args := readerLaunchCommand("/books/alpha.bookpkg", "")
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}
