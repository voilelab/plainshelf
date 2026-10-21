package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBookPathFromArgs(t *testing.T) {
	tests := map[string]struct {
		args []string
		want string
	}{
		"no arguments":            {args: nil, want: ""},
		"separate value":          {args: []string{"-book", "/books/dune.bookpkg"}, want: "/books/dune.bookpkg"},
		"joined value":            {args: []string{"-book=/books/dune.bookpkg"}, want: "/books/dune.bookpkg"},
		"double dash":             {args: []string{"--book", "dune.bookpkg"}, want: "dune.bookpkg"},
		"double dash joined":      {args: []string{"--book=dune.bookpkg"}, want: "dune.bookpkg"},
		"a bare path is not used": {args: []string{"/books/dune.bookpkg"}, want: ""},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			launch, err := parseLaunchArgs(test.args)
			if err != nil {
				t.Fatalf("parseLaunchArgs(%q): %v", test.args, err)
			}
			if got := launch.bookPath; got != test.want {
				t.Errorf("parseLaunchArgs(%q).bookPath = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestParseLaunchArgs(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantBook    string
		wantShelf   string
		wantSection int
	}{
		"neither flag":            {args: nil, wantSection: noLaunchSection},
		"shelf only":              {args: []string{"-shelf", "shelf-real"}, wantShelf: "shelf-real", wantSection: noLaunchSection},
		"book then shelf":         {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"shelf then book":         {args: []string{"-shelf", "shelf-real", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"joined values":           {args: []string{"-book=/books/dune.bookpkg", "-shelf=shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"book, shelf and section": {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real", "-section", "4"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: 4},
		"section then book":       {args: []string{"-section", "4", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantSection: 4},
		"section zero":            {args: []string{"-book", "/books/dune.bookpkg", "-section", "0"}, wantBook: "/books/dune.bookpkg", wantSection: 0},
		"negative section":        {args: []string{"-section=-2"}, wantSection: -2},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseLaunchArgs(test.args)
			if err != nil {
				t.Fatalf("parseLaunchArgs(%q): %v", test.args, err)
			}
			if got.bookPath != test.wantBook {
				t.Errorf("parseLaunchArgs(%q).bookPath = %q, want %q", test.args, got.bookPath, test.wantBook)
			}
			if got.shelfID != test.wantShelf {
				t.Errorf("parseLaunchArgs(%q).shelfID = %q, want %q", test.args, got.shelfID, test.wantShelf)
			}
			if got.section != test.wantSection {
				t.Errorf("parseLaunchArgs(%q).section = %d, want %d", test.args, got.section, test.wantSection)
			}
		})
	}
}

func TestParseLaunchArgsRejectsInvalidFlagsWithoutPartialResults(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown flag":                      {"-loglevel", "debug"},
		"unknown flag before book":          {"-loglevel", "debug", "-book", "/books/dune.bookpkg"},
		"unknown flag after book":           {"-book", "/books/dune.bookpkg", "-unknown"},
		"book without value":                {"-book"},
		"shelf without value":               {"-shelf"},
		"section without value":             {"-section"},
		"non-integer section":               {"-section", "chapter-one"},
		"overflowing section":               {"-section=99999999999999999999999999"},
		"invalid section after valid flags": {"-book", "/books/dune.bookpkg", "-shelf", "shelf-real", "-section=bad"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseLaunchArgs(args)
			if err == nil {
				t.Fatalf("parseLaunchArgs(%q) succeeded, want an error", args)
			}
			if got != (launchArgs{}) {
				t.Fatalf("parseLaunchArgs(%q) returned partial launch arguments: %+v", args, got)
			}
		})
	}
}

func TestMainRejectsInvalidLaunchArgumentsBeforeStartup(t *testing.T) {
	const child = "PLAINSHELF_READER_TEST_INVALID_ARGS"
	if os.Getenv(child) == "1" {
		os.Args = []string{"plainshelf-reader", "-section=chapter-one"}
		main()
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainRejectsInvalidLaunchArgumentsBeforeStartup$")
	cmd.Env = append(os.Environ(), child+"=1")
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("invalid launch exited with %v, want exit code 1; output: %s", err, output)
	}
	if !strings.Contains(string(output), "invalid reader launch arguments:") || !strings.Contains(string(output), "-section") {
		t.Fatalf("invalid launch did not report the flag error: %s", output)
	}
}
