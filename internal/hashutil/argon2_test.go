package hashutil

import (
	"strings"
	"testing"
)

func TestSnapshotContentHashPHC(t *testing.T) {
	content := []byte("same-content")

	hashA, err := NewContentHash(content)
	if err != nil {
		t.Fatalf("Failed to create snapshot content hash A: %v", err)
	}
	hashB, err := NewContentHash(content)
	if err != nil {
		t.Fatalf("Failed to create snapshot content hash B: %v", err)
	}

	if !strings.HasPrefix(hashA, "$argon2id$") {
		t.Fatalf("Expected PHC prefix '$argon2id$', got '%s'", hashA)
	}
	if hashA == hashB {
		t.Fatalf("Expected different hashes for same content due to random salt")
	}

	ok, err := VerifyContentHash(content, hashA)
	if err != nil {
		t.Fatalf("Failed to verify snapshot content hash: %v", err)
	}
	if !ok {
		t.Fatalf("Expected hash verification to succeed")
	}

	ok, err = VerifyContentHash([]byte("different-content"), hashA)
	if err != nil {
		t.Fatalf("Failed to verify mismatched snapshot content hash: %v", err)
	}
	if ok {
		t.Fatalf("Expected hash verification to fail for different content")
	}
}
