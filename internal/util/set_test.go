package util

import "testing"

func TestSetNewIsEmpty(t *testing.T) {
	s := NewSet[int]()

	if got := len(s.Items()); got != 0 {
		t.Fatalf("expected empty set, got len=%d", got)
	}
}

func TestSetAddContainsAndRemove(t *testing.T) {
	s := NewSet[string]()

	s.Add("alpha")
	s.Add("beta")

	if !s.Contains("alpha") {
		t.Fatalf("expected set to contain alpha")
	}
	if !s.Contains("beta") {
		t.Fatalf("expected set to contain beta")
	}
	if s.Contains("gamma") {
		t.Fatalf("did not expect set to contain gamma")
	}

	s.Remove("alpha")

	if s.Contains("alpha") {
		t.Fatalf("did not expect set to contain alpha after remove")
	}
	if !s.Contains("beta") {
		t.Fatalf("expected set to still contain beta")
	}
}

func TestSetItemsUniqueElements(t *testing.T) {
	s := NewSet[int]()

	s.Add(1)
	s.Add(1)
	s.Add(2)

	items := s.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 unique items, got %d", len(items))
	}

	found := map[int]bool{}
	for _, item := range items {
		found[item] = true
	}

	if !found[1] || !found[2] {
		t.Fatalf("expected items to contain 1 and 2, got %+v", items)
	}
}
