package server

import (
	"errors"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

func ptr[T any](v T) *T { return &v }

// A PATCH must leave out what the request left out, which is the whole reason
// UpdateBookRequest uses pointers. An empty request may touch nothing but the
// update timestamp.
func TestApplyBookPatchLeavesOmittedFieldsAlone(t *testing.T) {
	before := shelf.BookMeta{
		ID:       "book_1",
		Title:    "Original",
		Authors:  []string{"Author"},
		Tags:     []string{"tag"},
		Language: "en",
		Comments: "note",
		Star:     3,
	}

	meta := before
	if err := applyBookPatch(&meta, &UpdateBookRequest{}); err != nil {
		t.Fatalf("applyBookPatch: %v", err)
	}

	meta.UpdatedAt = before.UpdatedAt
	if meta.ID != before.ID || meta.Title != before.Title || meta.Language != before.Language ||
		meta.Comments != before.Comments || meta.Star != before.Star {
		t.Fatalf("meta = %+v, want it unchanged from %+v", meta, before)
	}
}

func TestApplyBookPatchAppliesSetFields(t *testing.T) {
	meta := shelf.BookMeta{Title: "Original", Star: 1}
	published := util.JSONDate(time.Date(2020, 3, 4, 0, 0, 0, 0, time.UTC))

	req := UpdateBookRequest{
		Title:       ptr("New Title"),
		Authors:     ptr([]string{"A", "B"}),
		Tags:        ptr([]string{"t"}),
		Identifiers: ptr(map[string]string{"isbn": "123"}),
		Language:    ptr("zh"),
		Comment:     ptr("updated note"),
		Star:        ptr(5),
		PublishedAt: &published,
	}

	if err := applyBookPatch(&meta, &req); err != nil {
		t.Fatalf("applyBookPatch: %v", err)
	}

	if meta.Title != "New Title" || meta.Language != "zh" || meta.Comments != "updated note" || meta.Star != 5 {
		t.Fatalf("scalar fields not applied: %+v", meta)
	}
	if len(meta.Authors) != 2 || meta.Authors[0] != "A" || meta.Authors[1] != "B" {
		t.Fatalf("authors = %v, want [A B]", meta.Authors)
	}
	if meta.Identifiers["isbn"] != "123" {
		t.Fatalf("identifiers = %v, want isbn=123", meta.Identifiers)
	}
	if time.Time(meta.PublishedAt) != time.Time(published) {
		t.Fatalf("published_at = %v, want %v", meta.PublishedAt, published)
	}
	if time.Time(meta.UpdatedAt).IsZero() {
		t.Fatal("updated_at was not stamped")
	}
}

// The slice fields are copied, not aliased: a decoded request must not stay
// reachable from the meta that gets written to disk.
func TestApplyBookPatchCopiesSliceFields(t *testing.T) {
	authors := []string{"Original Author"}
	tags := []string{"original"}

	meta := shelf.BookMeta{}
	if err := applyBookPatch(&meta, &UpdateBookRequest{Authors: &authors, Tags: &tags}); err != nil {
		t.Fatalf("applyBookPatch: %v", err)
	}

	authors[0] = "Mutated"
	tags[0] = "mutated"

	if meta.Authors[0] != "Original Author" {
		t.Fatalf("authors aliased the request slice: %v", meta.Authors)
	}
	if meta.Tags[0] != "original" {
		t.Fatalf("tags aliased the request slice: %v", meta.Tags)
	}
}

func TestApplyBookPatchRejectsStarOutOfRange(t *testing.T) {
	for _, star := range []int{-1, 6} {
		meta := shelf.BookMeta{Star: 3}
		err := applyBookPatch(&meta, &UpdateBookRequest{Star: &star})

		if !errors.Is(err, errStarOutOfRange) {
			t.Fatalf("star %d: err = %v, want errStarOutOfRange", star, err)
		}
	}

	// The bounds themselves are accepted.
	for _, star := range []int{minBookStar, maxBookStar} {
		meta := shelf.BookMeta{}
		if err := applyBookPatch(&meta, &UpdateBookRequest{Star: &star}); err != nil {
			t.Fatalf("star %d: %v", star, err)
		}
		if meta.Star != star {
			t.Fatalf("star = %d, want %d", meta.Star, star)
		}
	}
}

// "layers" predates "layer"; both are still accepted, with "layer" winning.
func TestUpdateBookRequestTargetLayers(t *testing.T) {
	layer := shelf.Layers{"from-layer"}
	legacy := shelf.Layers{"from-layers"}

	tests := []struct {
		name string
		req  UpdateBookRequest
		want *shelf.Layers
	}{
		{"neither set", UpdateBookRequest{}, nil},
		{"layer only", UpdateBookRequest{Layer: &layer}, &layer},
		{"layers only", UpdateBookRequest{Layers: &legacy}, &legacy},
		{"layer wins over layers", UpdateBookRequest{Layer: &layer, Layers: &legacy}, &layer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.targetLayers(); got != tt.want {
				t.Fatalf("targetLayers() = %v, want %v", got, tt.want)
			}
		})
	}
}
