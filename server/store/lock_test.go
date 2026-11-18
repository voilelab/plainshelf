//go:build !(js && wasm)

package store

import (
	"os"
	"os/exec"
	"testing"
)

func TestStoreExclusiveLock(t *testing.T) {
	const childEnv = "PLAINSHELF_STORE_LOCK_TEST"
	if dir := os.Getenv(childEnv); dir != "" {
		if db, err := New(dir); err == nil {
			db.Close()
			t.Fatal("second process opened a locked store")
		}
		return
	}
	dir := t.TempDir()
	db := newTestDBAt(t, dir)
	if second, err := New(dir); err == nil {
		second.Close()
		t.Fatal("second instance opened a locked store")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStoreExclusiveLock$")
	cmd.Env = append(os.Environ(), childEnv+"="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	newTestDBAt(t, dir)
}
