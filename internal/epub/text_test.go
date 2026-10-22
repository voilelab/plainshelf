package epub

import "testing"

func TestDocumentPreservesNBSPBetweenCJKButDropsLineWrap(t *testing.T) {
	for _, tc := range []struct {
		name string
		html string
		want string
	}{
		{"nbsp between CJK survives", "<p>甲&nbsp;乙</p>", "甲 乙"},
		{"line wrap between CJK is dropped", "<p>甲\n乙</p>", "甲乙"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got, _ := documentToChapter([]byte(tc.html), Options{}, nil)
			if got != tc.want {
				t.Errorf("document body = %q, want %q", got, tc.want)
			}
		})
	}
}
