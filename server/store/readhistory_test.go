package store

import (
	"fmt"
	"testing"
)

func TestGetReadHistory_NotFound(t *testing.T) {
	db := newTestDB(t)
	history, err := db.GetReadHistory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %v", history)
	}
}

func TestSetReadHistory(t *testing.T) {
	db := newTestDB(t)
	history := []string{"book1", "book2", "book3"}
	if err := db.SetReadHistory(history); err != nil {
		t.Fatalf("SetReadHistory: %v", err)
	}
	got, err := db.GetReadHistory()
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}

	if len(got) != len(history) {
		t.Fatalf("expected history length %d, got %d", len(history), len(got))
	}
	for i := range history {
		if got[i] != history[i] {
			t.Fatalf("expected history[%d] = %s, got %s", i, history[i], got[i])
		}
	}
}

func TestAddToReadHistory(t *testing.T) {
	db := newTestDB(t)
	if err := db.AddToReadHistory("book1"); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	if err := db.AddToReadHistory("book2"); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	if err := db.AddToReadHistory("book1"); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	history, err := db.GetReadHistory()
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}
	expected := []string{"book1", "book2"}
	if len(history) != len(expected) {
		t.Fatalf("expected history length %d, got %d", len(expected), len(history))
	}
	for i := range expected {
		if history[i] != expected[i] {
			t.Fatalf("expected history[%d] = %s, got %s", i, expected[i], history[i])
		}
	}
}

func TestAddToReadHistory_ExceedLimit(t *testing.T) {
	db := newTestDB(t)
	for i := 1; i <= 150; i++ {
		bookID := fmt.Sprintf("book%d", i)
		if err := db.AddToReadHistory(bookID); err != nil {
			t.Fatalf("AddToReadHistory: %v", err)
		}
	}
	history, err := db.GetReadHistory()
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}
	if len(history) != db.readHistoryLimit {
		t.Fatalf("expected history length %d, got %d", db.readHistoryLimit, len(history))
	}
	for i := 150; i > 50; i-- {
		expectedID := fmt.Sprintf("book%d", i)
		if history[150-i] != expectedID {
			t.Fatalf("expected history[%d] = %s, got %s", 150-i, expectedID, history[150-i])
		}
	}
}
