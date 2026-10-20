package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

func TestReaderUnsupportedPlatformCodeMatchesFrontend(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "frontend", "src", "api", "desktop.ts"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^const READER_UNSUPPORTED_PLATFORM_CODE = '([^']+)';$`).FindSubmatch(source)
	if len(match) != 2 {
		t.Fatal("frontend reader unsupported-platform code declaration not found")
	}
	if got := string(match[1]); got != readerUnsupportedPlatformCode {
		t.Fatalf("frontend code = %q, desktop code = %q", got, readerUnsupportedPlatformCode)
	}
}

func TestReaderLaunchCommandDefault(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	name, args := readerLaunchCommand("/books/alpha.bookpkg", "shelf-real", -1)
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

	_, args := readerLaunchCommand("/books/beta.bookpkg", "shelf-real", -1)
	want := []string{"-n", "-a", "/tmp/build/PlainShelfReader.app", "--args", "-book", "/books/beta.bookpkg", "-shelf", "shelf-real"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestReaderLaunchCommandNoShelf(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	_, args := readerLaunchCommand("/books/alpha.bookpkg", "", -1)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestReaderLaunchCommandWithSection(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	_, args := readerLaunchCommand("/books/alpha.bookpkg", "shelf-real", 3)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-shelf", "shelf-real", "-section", "3"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestReaderLaunchCommandSectionZero(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	_, args := readerLaunchCommand("/books/alpha.bookpkg", "", 0)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-section", "0"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}
