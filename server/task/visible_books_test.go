package task

import "testing"

func TestVisibleBooksAllowsOnlySelectedBooksUnlessAll(t *testing.T) {
	for _, tt := range []struct {
		name       string
		visibility VisibleBooks
		want       bool
	}{
		{"all permits every book", AllBooks(), true},
		{"zero value permits no book", VisibleBooks{}, false},
		{"empty selection permits no book", OnlyBooks(), false},
		{"selected permits the book", OnlyBooks("book"), true},
		{"unselected denies the book", OnlyBooks("other"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.visibility.Allows("book"); got != tt.want {
				t.Fatalf("Allows(book) = %v, want %v", got, tt.want)
			}
		})
	}
}
