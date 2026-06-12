package store

import (
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func makeShelf(id string) *shelf.ShelfConfWithID {
	return &shelf.ShelfConfWithID{
		ID:   id,
		Name: "Shelf " + id,
	}
}

func TestGetShelves_Empty(t *testing.T) {
	db := newTestDB(t)
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != 0 {
		t.Fatalf("expected 0 shelves, got %d", len(shelves))
	}
}

func TestAddShelf(t *testing.T) {
	db := newTestDB(t)
	if err := db.AddShelf(makeShelf("s1")); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != 1 {
		t.Fatalf("expected 1 shelf, got %d", len(shelves))
	}
	if shelves[0].ID != "s1" {
		t.Fatalf("expected ID %q, got %q", "s1", shelves[0].ID)
	}
}

func TestAddShelf_DuplicateID(t *testing.T) {
	db := newTestDB(t)
	db.AddShelf(makeShelf("s1"))
	if err := db.AddShelf(makeShelf("s1")); err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestAddShelf_Multiple(t *testing.T) {
	db := newTestDB(t)
	ids := []string{"a", "b", "c"}
	for _, id := range ids {
		if err := db.AddShelf(makeShelf(id)); err != nil {
			t.Fatalf("AddShelf %q: %v", id, err)
		}
	}
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != len(ids) {
		t.Fatalf("expected %d shelves, got %d", len(ids), len(shelves))
	}
}

func TestDeleteShelf(t *testing.T) {
	db := newTestDB(t)
	db.AddShelf(makeShelf("s1"))
	if err := db.DeleteShelf("s1"); err != nil {
		t.Fatalf("DeleteShelf: %v", err)
	}
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != 0 {
		t.Fatalf("expected 0 shelves after delete, got %d", len(shelves))
	}
}

func TestDeleteShelf_LeavesOthers(t *testing.T) {
	db := newTestDB(t)
	db.AddShelf(makeShelf("s1"))
	db.AddShelf(makeShelf("s2"))
	if err := db.DeleteShelf("s1"); err != nil {
		t.Fatalf("DeleteShelf: %v", err)
	}
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != 1 {
		t.Fatalf("expected 1 shelf remaining, got %d", len(shelves))
	}
	if shelves[0].ID != "s2" {
		t.Fatalf("expected remaining shelf ID %q, got %q", "s2", shelves[0].ID)
	}
}

func TestAddShelf_AfterDelete(t *testing.T) {
	db := newTestDB(t)
	db.AddShelf(makeShelf("s1"))
	db.DeleteShelf("s1")
	if err := db.AddShelf(makeShelf("s1")); err != nil {
		t.Fatalf("AddShelf after delete: %v", err)
	}
	shelves, err := db.GetShelves()
	if err != nil {
		t.Fatalf("GetShelves: %v", err)
	}
	if len(shelves) != 1 {
		t.Fatalf("expected 1 shelf, got %d", len(shelves))
	}
}
