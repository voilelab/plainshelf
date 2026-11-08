package server

import (
	"strconv"
	"testing"
)

func pageHashes(from, to int) []string {
	hashes := []string{}
	for n := from; n <= to; n++ {
		hashes = append(hashes, strconv.Itoa(1000+n))
	}
	return hashes
}

func TestComicOverlapsShareThreshold(t *testing.T) {
	tests := []struct {
		name   string
		b      []string
		shared int
	}{
		{name: "nine of ten shared", b: append(pageHashes(1, 9), pageHashes(50, 55)...), shared: 9},
		{name: "eight of ten shared", b: append(pageHashes(1, 8), pageHashes(50, 55)...), shared: 0},
		{name: "superset", b: pageHashes(1, 12), shared: 10},
		{name: "identical", b: pageHashes(1, 10), shared: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comics := []comicPages{{bookID: "a", hashes: pageHashes(1, 10)}, {bookID: "b", hashes: tc.b}}
			overlaps := comicOverlaps(comics)
			switch {
			case tc.shared == 0 && len(overlaps) != 0:
				t.Fatalf("overlaps = %+v, want none", overlaps)
			case tc.shared > 0 && (len(overlaps) != 1 || overlaps[0].Shared != tc.shared || overlaps[0].PagesB != len(tc.b)):
				t.Fatalf("overlaps = %+v, want one pair sharing %d pages", overlaps, tc.shared)
			}
		})
	}
}

func TestComicOverlapsPairExactCopiesOnce(t *testing.T) {
	comics := []comicPages{
		{bookID: "a", hashes: pageHashes(1, 10)},
		{bookID: "b", hashes: pageHashes(1, 10)},
		{bookID: "c", hashes: pageHashes(1, 11)},
	}
	if overlaps := comicOverlaps(comics); len(overlaps) != 1 || overlaps[0].A != "a" || overlaps[0].B != "c" {
		t.Fatalf("overlaps = %+v, want one pair between the first exact copy and the longer comic", overlaps)
	}
}

func TestExactComicGroupsSkipEmptyComics(t *testing.T) {
	groups := exactComicGroups([]comicPages{{bookID: "a"}, {bookID: "b"}, {bookID: "c", hashes: []string{"x"}}})
	if len(groups) != 0 {
		t.Fatalf("groups = %v, want comics without pages left ungrouped", groups)
	}
}
