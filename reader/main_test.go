package main

import "testing"

// The book path is a named flag rather than a positional argument because
// `wails dev` hands the app whatever -appargs carried, and a bare value there
// is as likely to belong to another flag as to be a path.
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
		"no value after -book":    {args: []string{"-book"}, want: ""},
		// An argument this app does not define ends the parse rather than
		// failing the launch. In practice it does not arrive: `wails dev` passes
		// its own settings to the binary through the environment, and only
		// -appargs reaches os.Args.
		"an undefined flag": {args: []string{"-loglevel", "debug"}, want: ""},
		"after an undefined flag": {
			args: []string{"-loglevel", "debug", "-book", "/books/dune.bookpkg"},
			want: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := bookPathFromArgs(test.args); got != test.want {
				t.Errorf("bookPathFromArgs(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// -shelf carries the real desktop shelf id and -section the chapter to open at.
// Both are parsed on the same FlagSet as -book so that any flag order the desktop
// app might emit is read whole — a FlagSet that knew only one of them would stop
// at the others. -section defaults to noLaunchSection (a negative sentinel) so
// section 0, the first chapter, stays distinguishable from "no chapter requested".
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
		"no value after -shelf":   {args: []string{"-shelf"}, wantSection: noLaunchSection},
		"book, shelf and section": {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real", "-section", "4"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real", wantSection: 4},
		"section then book":       {args: []string{"-section", "4", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantSection: 4},
		"section zero":            {args: []string{"-book", "/books/dune.bookpkg", "-section", "0"}, wantBook: "/books/dune.bookpkg", wantSection: 0},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := parseLaunchArgs(test.args)
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
