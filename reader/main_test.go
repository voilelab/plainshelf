package main

import (
	"context"
	"errors"
	"flag"
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
		"no arguments":               {args: nil, want: ""},
		"separate value":             {args: []string{"-book", "/books/dune.bookpkg"}, want: "/books/dune.bookpkg"},
		"joined value":               {args: []string{"-book=/books/dune.bookpkg"}, want: "/books/dune.bookpkg"},
		"double dash":                {args: []string{"--book", "dune.bookpkg"}, want: "dune.bookpkg"},
		"double dash joined":         {args: []string{"--book=dune.bookpkg"}, want: "dune.bookpkg"},
		"a bare path is not used":    {args: []string{"/books/dune.bookpkg"}, want: ""},
		"joined dash-leading path":   {args: []string{"-book=-section=4"}, want: "-section=4"},
		"double-dash joined path":    {args: []string{"--book=-notes.bookpkg"}, want: "-notes.bookpkg"},
		"relative dash-leading path": {args: []string{"-book", "./-notes.bookpkg"}, want: "./-notes.bookpkg"},
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
		"neither flag":              {args: nil, wantSection: noLaunchSection},
		"shelf only":                {args: []string{"-shelf", "shelf-real"}, wantShelf: "shelf-real", wantSection: noLaunchSection},
		"book then shelf":           {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"shelf then book":           {args: []string{"-shelf", "shelf-real", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"joined values":             {args: []string{"-book=/books/dune.bookpkg", "-shelf=shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: noLaunchSection},
		"book, shelf and section":   {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real", "-section", "4"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: 4},
		"section then book":         {args: []string{"-section", "4", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantSection: 4},
		"section zero":              {args: []string{"-book", "/books/dune.bookpkg", "-section", "0"}, wantBook: "/books/dune.bookpkg", wantSection: 0},
		"negative section":          {args: []string{"-section=-2"}, wantSection: -2},
		"separate negative section": {args: []string{"-section", "-2", "-shelf", "shelf-real"}, wantShelf: "shelf-real", wantSection: -2},
		"joined dash-leading shelf": {args: []string{"-shelf=-unknown"}, wantShelf: "-unknown", wantSection: noLaunchSection},
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
		"unknown flag":                            {"-loglevel", "debug"},
		"unknown flag before book":                {"-loglevel", "debug", "-book", "/books/dune.bookpkg"},
		"unknown flag after book":                 {"-book", "/books/dune.bookpkg", "-unknown"},
		"book without value":                      {"-book"},
		"shelf without value":                     {"-shelf"},
		"section without value":                   {"-section"},
		"non-integer section":                     {"-section", "chapter-one"},
		"overflowing section":                     {"-section=99999999999999999999999999"},
		"invalid section after valid flags":       {"-book", "/books/dune.bookpkg", "-shelf", "shelf-real", "-section=bad"},
		"book followed by section flag":           {"-book", "-section=4"},
		"book followed by double-dash shelf flag": {"-book", "--shelf=shelf-real"},
		"shelf followed by unknown flag":          {"-shelf", "-unknown"},
		"double-dash book followed by flag":       {"--book", "-section=4"},
		"double-dash shelf followed by flag":      {"--shelf", "--book=/books/dune.bookpkg"},
		"book followed by terminator":             {"-book", "--"},
		"missing shelf value after valid book":    {"-book", "/books/dune.bookpkg", "-shelf", "-unknown"},
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
		os.Args = append([]string{"plainshelf-reader"}, flag.Args()...)
		main()
		return
	}
	for _, tt := range []struct {
		name string
		args []string
		flag string
	}{
		{"invalid section", []string{"-section=chapter-one"}, "-section"},
		{"book swallowing section", []string{"-book", "-section=4"}, "-book"},
		{"shelf swallowing unknown flag", []string{"-shelf", "-unknown"}, "-shelf"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestMainRejectsInvalidLaunchArgumentsBeforeStartup$", "--"}, tt.args...)
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			cmd.Env = append(os.Environ(), child+"=1")
			output, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("invalid launch exited with %v, want exit code 1; output: %s", err, output)
			}
			if !strings.Contains(string(output), "invalid reader launch arguments:") || !strings.Contains(string(output), tt.flag) {
				t.Fatalf("invalid launch did not report the flag error: %s", output)
			}
		})
	}
}
