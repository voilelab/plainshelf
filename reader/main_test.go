package main

import "testing"

// The app does not own its whole command line — `wails dev` passes its own
// flags through to the binary — so the book path is named rather than
// positional, and anything else present is stepped over rather than guessed at.
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
		"alongside other flags":   {args: []string{"-loglevel", "debug", "-book", "/books/dune.bookpkg"}, want: "/books/dune.bookpkg"},
		"another flag's value":    {args: []string{"-loglevel", "debug"}, want: ""},
		"a bare path is not used": {args: []string{"/books/dune.bookpkg"}, want: ""},
		"no value after -book":    {args: []string{"-book"}, want: ""},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := bookPathFromArgs(test.args); got != test.want {
				t.Errorf("bookPathFromArgs(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}
