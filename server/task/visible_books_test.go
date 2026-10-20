package task

import "testing"

func TestVisibleBooksDistinguishesUnrestrictedFromEmpty(t *testing.T) {
	for _, tt := range []struct {
		name       string
		visibility VisibleBooks
		want       bool
	}{
		{"nil permits every book", nil, true},
		{"empty permits no book", VisibleBooks{}, false},
		{"selected permits the book", VisibleBooks{"book": true}, true},
		{"false denies the book", VisibleBooks{"book": false}, false},
		{"unselected denies the book", VisibleBooks{"other": true}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.visibility.Allows("book"); got != tt.want {
				t.Fatalf("Allows(book) = %v, want %v", got, tt.want)
			}
		})
	}
}
