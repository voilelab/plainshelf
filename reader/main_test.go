package main

import "testing"

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

		"an undefined flag": {args: []string{"-loglevel", "debug"}, want: ""},
		"after an undefined flag": {
			args: []string{"-loglevel", "debug", "-book", "/books/dune.bookpkg"},
			want: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := parseLaunchArgs(test.args).bookPath; got != test.want {
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
