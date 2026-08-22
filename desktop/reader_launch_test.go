package main

import (
	"reflect"
	"testing"
)

func TestReaderLaunchCommandDefault(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	name, args := readerLaunchCommand("/books/alpha.bookpkg")
	if name != "open" {
		t.Fatalf("launcher = %q, want open", name)
	}
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestReaderLaunchCommandEnvOverride(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "/tmp/build/PlainShelfReader.app")

	_, args := readerLaunchCommand("/books/beta.bookpkg")
	want := []string{"-n", "-a", "/tmp/build/PlainShelfReader.app", "--args", "-book", "/books/beta.bookpkg"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}
