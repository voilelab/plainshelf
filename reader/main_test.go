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

// -shelf carries the real desktop shelf id. It is parsed on the same FlagSet as
// -book so that either flag order the desktop app might emit is read whole — a
// FlagSet that knew only one of them would stop at the other.
func TestParseLaunchArgs(t *testing.T) {
	tests := map[string]struct {
		args      []string
		wantBook  string
		wantShelf string
	}{
		"neither flag":          {args: nil},
		"shelf only":            {args: []string{"-shelf", "shelf-real"}, wantShelf: "shelf-real"},
		"book then shelf":       {args: []string{"-book", "/books/dune.bookpkg", "-shelf", "shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real"},
		"shelf then book":       {args: []string{"-shelf", "shelf-real", "-book", "/books/dune.bookpkg"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real"},
		"joined values":         {args: []string{"-book=/books/dune.bookpkg", "-shelf=shelf-real"}, wantBook: "/books/dune.bookpkg", wantShelf: "shelf-real"},
		"no value after -shelf": {args: []string{"-shelf"}},
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
		})
	}
}
