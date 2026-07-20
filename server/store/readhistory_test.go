package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestGetReadHistory_NotFound(t *testing.T) {
	db := newTestDB(t)
	history, err := db.GetReadHistory("/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %v", history)
	}
}

func TestSetReadHistory(t *testing.T) {
	db := newTestDB(t)
	dbID := "test_shelf"
	history := []string{"book1", "book2", "book3"}
	readHistoryLimit := 100
	if err := db.SetReadHistory(dbID, history, readHistoryLimit); err != nil {
		t.Fatalf("SetReadHistory: %v", err)
	}
	got, err := db.GetReadHistory(dbID)
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
	dbID := "test_shelf"
	readHistoryLimit := 100
	if err := db.AddToReadHistory(dbID, "book1", readHistoryLimit); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	if err := db.AddToReadHistory(dbID, "book2", readHistoryLimit); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	if err := db.AddToReadHistory(dbID, "book1", readHistoryLimit); err != nil {
		t.Fatalf("AddToReadHistory: %v", err)
	}
	history, err := db.GetReadHistory(dbID)
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
	dbID := "test_shelf"
	readHistoryLimit := 100
	for i := 1; i <= 150; i++ {
		bookID := fmt.Sprintf("book%d", i)
		if err := db.AddToReadHistory(dbID, bookID, readHistoryLimit); err != nil {
			t.Fatalf("AddToReadHistory: %v", err)
		}
	}
	history, err := db.GetReadHistory(dbID)
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}
	if len(history) != readHistoryLimit {
		t.Fatalf("expected history length %d, got %d", readHistoryLimit, len(history))
	}
	for i := 150; i > 50; i-- {
		expectedID := fmt.Sprintf("book%d", i)
		if history[150-i] != expectedID {
			t.Fatalf("expected history[%d] = %s, got %s", 150-i, expectedID, history[150-i])
		}
	}
}

func TestSetReadHistory_NegativeLimitIsEmpty(t *testing.T) {
	db := newTestDB(t)
	if err := db.SetReadHistory("test_shelf", []string{"book1"}, -1); err != nil {
		t.Fatalf("SetReadHistory: %v", err)
	}

	history, err := db.GetReadHistory("test_shelf")
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %v", history)
	}
}

func TestAddToReadHistory_ConcurrentUpdatesAreNotLost(t *testing.T) {
	db := newTestDB(t)
	const count = 12

	start := make(chan struct{})
	errCh := make(chan error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- db.AddToReadHistory("test_shelf", fmt.Sprintf("book%d", i), count)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("AddToReadHistory: %v", err)
		}
	}

	history, err := db.GetReadHistory("test_shelf")
	if err != nil {
		t.Fatalf("GetReadHistory: %v", err)
	}
	if len(history) != count {
		t.Fatalf("expected %d history entries, got %d: %v", count, len(history), history)
	}

	seen := make(map[string]bool, count)
	for _, bookID := range history {
		seen[bookID] = true
	}
	for i := range count {
		bookID := fmt.Sprintf("book%d", i)
		if !seen[bookID] {
			t.Errorf("missing %s from history: %v", bookID, history)
		}
	}
}
