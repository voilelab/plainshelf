package shelfmobile

import (
	"encoding/json"
	"testing"
	"time"
)

func TestShelfMobileVerticalSlice(t *testing.T) {
	root := t.TempDir()

	mobileShelf, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer mobileShelf.Close()

	createdJSON, err := mobileShelf.CreateBookJSON(`{"title":"Mobile Book","layers":["drafts"]}`)
	if err != nil {
		t.Fatalf("CreateBookJSON() error = %v", err)
	}
	created := decodeJSON[MobileBook](t, createdJSON)
	if created.ID == "" || created.Title != "Mobile Book" {
		t.Fatalf("unexpected created book: %+v", created)
	}
	if len(created.Layers) != 1 || created.Layers[0] != "drafts" {
		t.Fatalf("unexpected created layers: %+v", created.Layers)
	}

	booksJSON, err := mobileShelf.ListBooksJSON()
	if err != nil {
		t.Fatalf("ListBooksJSON() error = %v", err)
	}
	books := decodeJSON[[]MobileBook](t, booksJSON)
	if len(books) != 1 || books[0].ID != created.ID {
		t.Fatalf("unexpected books: %+v", books)
	}

	bookJSON, err := mobileShelf.GetBookJSON(created.ID)
	if err != nil {
		t.Fatalf("GetBookJSON() error = %v", err)
	}
	book := decodeJSON[MobileBook](t, bookJSON)
	if book.ID != created.ID {
		t.Fatalf("unexpected book: %+v", book)
	}

	sourceJSON, err := mobileShelf.CreateSourceJSON(created.ID)
	if err != nil {
		t.Fatalf("CreateSourceJSON() error = %v", err)
	}
	source := decodeJSON[MobileSource](t, sourceJSON)
	if source.ID == "" {
		t.Fatalf("expected source ID, got %+v", source)
	}

	sourcesJSON, err := mobileShelf.ListSourcesJSON(created.ID)
	if err != nil {
		t.Fatalf("ListSourcesJSON() error = %v", err)
	}
	sources := decodeJSON[[]MobileSource](t, sourcesJSON)
	if len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("unexpected sources: %+v", sources)
	}

	if err := mobileShelf.UpdateSourceContent(created.ID, source.ID, "hello\nmobile shelf"); err != nil {
		t.Fatalf("UpdateSourceContent() error = %v", err)
	}
	content, err := mobileShelf.GetSourceContent(created.ID, source.ID)
	if err != nil {
		t.Fatalf("GetSourceContent() error = %v", err)
	}
	if content != "hello\nmobile shelf" {
		t.Fatalf("unexpected source content: %q", content)
	}

	publishedAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC).Format(time.RFC3339)
	patch, err := json.Marshal(UpdateBookRequest{
		Title:       stringPtr("Updated Mobile Book"),
		Authors:     stringSlicePtr([]string{"Ada", "Lin"}),
		Tags:        stringSlicePtr([]string{"mobile", "mvp"}),
		Language:    stringPtr("en"),
		Comments:    stringPtr("edited from mobile"),
		PublishedAt: stringPtr(publishedAt),
	})
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}
	updatedJSON, err := mobileShelf.UpdateBookJSON(created.ID, string(patch))
	if err != nil {
		t.Fatalf("UpdateBookJSON() error = %v", err)
	}
	updated := decodeJSON[MobileBook](t, updatedJSON)
	if updated.Title != "Updated Mobile Book" || updated.Language != "en" || updated.PublishedAt != publishedAt {
		t.Fatalf("unexpected updated book: %+v", updated)
	}
	if len(updated.Authors) != 2 || updated.Authors[0] != "Ada" || len(updated.Tags) != 2 || updated.Tags[1] != "mvp" {
		t.Fatalf("unexpected updated slices: %+v %+v", updated.Authors, updated.Tags)
	}

	movedJSON, err := mobileShelf.MoveBookJSON(created.ID, `{"layers":["archive","mobile"]}`)
	if err != nil {
		t.Fatalf("MoveBookJSON() error = %v", err)
	}
	moved := decodeJSON[MobileBook](t, movedJSON)
	if len(moved.Layers) != 2 || moved.Layers[0] != "archive" || moved.Layers[1] != "mobile" {
		t.Fatalf("unexpected moved layers: %+v", moved.Layers)
	}

	if err := mobileShelf.MoveBookToTrash(created.ID); err != nil {
		t.Fatalf("MoveBookToTrash() error = %v", err)
	}
	booksJSON, err = mobileShelf.ListBooksJSON()
	if err != nil {
		t.Fatalf("ListBooksJSON() after trash error = %v", err)
	}
	books = decodeJSON[[]MobileBook](t, booksJSON)
	if len(books) != 0 {
		t.Fatalf("expected no active books after trash, got %+v", books)
	}
}

func decodeJSON[T any](t *testing.T, raw string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return value
}

func stringPtr(v string) *string { return &v }

func stringSlicePtr(v []string) *[]string { return &v }
