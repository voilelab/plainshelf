package server

import (
	"cmp"
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

const (
	overlapShareNumerator   = 9
	overlapShareDenominator = 10
)

type comicPages struct {
	bookID string
	hashes []string
}

type comicOverlap struct {
	A string `json:"a"`
	B string `json:"b"`

	PagesA int `json:"pages_a"`
	PagesB int `json:"pages_b"`
	Shared int `json:"shared"`
}

type comicOverlapReport struct {
	Total    int            `json:"total"`
	Hashed   int            `json:"hashed"`
	Missing  int            `json:"missing"`
	Overlaps []comicOverlap `json:"overlaps"`
}

func recordedComicPages(cache *shelf.PageHashCache, books []shelf.BookListing) (comics []comicPages, total int) {
	for _, listing := range books {
		if listing.SourceFormat != shelf.BookFormatImages {
			continue
		}
		total++
		book := listing.Book
		source, err := book.GetSource(book.CurrentSource())
		if err != nil {
			continue
		}
		hashes, ok := cache.Recorded(book.ID(), source.ID(), source.GetMeta().MD5Hash)
		if !ok {
			continue
		}
		comics = append(comics, comicPages{bookID: book.ID(), hashes: hashes})
	}
	slices.SortFunc(comics, func(a, b comicPages) int { return cmp.Compare(a.bookID, b.bookID) })
	return comics, total
}

func exactComicGroups(comics []comicPages) [][]string {
	byPages := map[string][]string{}
	var order []string
	for _, comic := range comics {
		if len(comic.hashes) == 0 {
			continue
		}
		key := strings.Join(comic.hashes, ",")
		if _, seen := byPages[key]; !seen {
			order = append(order, key)
		}
		byPages[key] = append(byPages[key], comic.bookID)
	}

	groups := [][]string{}
	for _, key := range order {
		if ids := byPages[key]; len(ids) > 1 {
			groups = append(groups, ids)
		}
	}
	return groups
}

func distinctPageSets(comics []comicPages) []comicPages {
	seen := map[string]bool{}
	return slices.DeleteFunc(slices.Clone(comics), func(comic comicPages) bool {
		key := strings.Join(comic.hashes, ",")
		duplicate := seen[key]
		seen[key] = true
		return duplicate
	})
}

func comicOverlaps(comics []comicPages) []comicOverlap {
	comics = distinctPageSets(comics)
	holders := map[string][]int{}
	for i, comic := range comics {
		for _, hash := range comic.hashes {
			holders[hash] = append(holders[hash], i)
		}
	}

	shared := map[[2]int]int{}
	for _, indexes := range holders {
		for x := range indexes {
			for y := x + 1; y < len(indexes); y++ {
				shared[[2]int{indexes[x], indexes[y]}]++
			}
		}
	}

	overlaps := []comicOverlap{}
	for pair, count := range shared {
		a, b := comics[pair[0]], comics[pair[1]]
		smaller := min(len(a.hashes), len(b.hashes))
		if count*overlapShareDenominator < smaller*overlapShareNumerator {
			continue
		}
		overlaps = append(overlaps, comicOverlap{
			A: a.bookID, B: b.bookID,
			PagesA: len(a.hashes), PagesB: len(b.hashes),
			Shared: count,
		})
	}
	slices.SortFunc(overlaps, func(x, y comicOverlap) int {
		return cmp.Or(cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})
	return overlaps
}

func (h *bookHandlers) findComicOverlaps(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	books, err := h.visibility(shelfData).listBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	comics, total := recordedComicPages(shelfData.OpenPageHashCache(), books)
	h.writeJSON(w, http.StatusOK, comicOverlapReport{
		Total:    total,
		Hashed:   len(comics),
		Missing:  total - len(comics),
		Overlaps: comicOverlaps(comics),
	})
}

func (h *batchHandlers) hashComicPages(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	visible, err := h.visibility(shelfData).visibleBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewHashComicPagesChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r), visible),
		"failed to schedule comic page hash task")
}
