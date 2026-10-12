package store

import "testing"

func newTestDB(t *testing.T) *DB {
	t.Helper()
	return newTestDBAt(t, t.TempDir())
}

func newTestDBAt(t *testing.T, dir string) *DB {
	t.Helper()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return db
}
