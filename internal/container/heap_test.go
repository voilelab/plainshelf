package container

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// checkInvariant fails if any element sorts before its parent.
func checkInvariant[T any](t *testing.T, heap *Heap[T]) {
	t.Helper()

	items := heap.Items()
	for i := 1; i < len(items); i++ {
		if heap.less(items[i], items[(i-1)/2]) {
			t.Fatalf("heap property broken at index %d: %v before parent %v", i, items[i], items[(i-1)/2])
		}
	}
}

func TestPopReturnsElementsInOrder(t *testing.T) {
	values := []int{5, 1, 9, 1, 7, 3, 8, 0, 2, 6}

	testCases := []struct {
		name string
		less func(a, b int) bool
		want []int
	}{
		{name: "min-heap", less: Less[int], want: []int{0, 1, 1, 2, 3, 5, 6, 7, 8, 9}},
		{name: "max-heap", less: Greater[int], want: []int{9, 8, 7, 6, 5, 3, 2, 1, 1, 0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			heap := New(tc.less)
			for _, value := range values {
				heap.Push(value)
				checkInvariant(t, heap)
			}

			var got []int
			for heap.Len() > 0 {
				root, ok := heap.Pop()
				if !ok {
					t.Fatal("Pop reported empty while Len is positive")
				}
				checkInvariant(t, heap)
				got = append(got, root)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("popped %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFromHeapifiesInPlace(t *testing.T) {
	items := []int{5, 1, 9, 1, 7, 3, 8, 0, 2, 6}
	heap := From(items, Less[int])

	checkInvariant(t, heap)
	if heap.Len() != len(items) {
		t.Fatalf("Len is %d, want %d", heap.Len(), len(items))
	}

	root, ok := heap.Peek()
	if !ok || root != 0 {
		t.Errorf("Peek returned (%d, %t), want (0, true)", root, ok)
	}
}

func TestFromEmptySlicePreallocates(t *testing.T) {
	heap := From(make([]int, 0, 64), Less[int])

	if heap.Len() != 0 {
		t.Fatalf("Len is %d, want 0", heap.Len())
	}
	for i := range 64 {
		heap.Push(i)
	}
	if capacity := cap(heap.Items()); capacity != 64 {
		t.Errorf("capacity is %d after 64 pushes, want the preallocated 64", capacity)
	}
}

func TestEmptyHeap(t *testing.T) {
	heap := New(Less[string])

	if _, ok := heap.Peek(); ok {
		t.Error("Peek reported a root on an empty heap")
	}
	if _, ok := heap.Pop(); ok {
		t.Error("Pop reported a root on an empty heap")
	}

	displaced, ok := heap.Replace("first")
	if ok || displaced != "" {
		t.Errorf(`Replace on an empty heap returned (%q, %t), want ("", false)`, displaced, ok)
	}
	if root, _ := heap.Peek(); root != "first" {
		t.Errorf("Replace on an empty heap left root %q, want it pushed", root)
	}
}

func TestReplaceMatchesPopThenPush(t *testing.T) {
	values := []int{5, 1, 9, 7, 3}

	replaced := From(slices.Clone(values), Less[int])
	displaced, ok := replaced.Replace(4)
	if !ok || displaced != 1 {
		t.Fatalf("Replace returned (%d, %t), want (1, true)", displaced, ok)
	}
	checkInvariant(t, replaced)

	popped := From(slices.Clone(values), Less[int])
	popped.Pop()
	popped.Push(4)

	if !slices.Equal(drain(replaced), drain(popped)) {
		t.Error("Replace and Pop followed by Push left different contents")
	}
}

// TestBoundedSmallest exercises the bottom-k use the max-heap exists for: keep
// the k smallest of a stream by evicting the root whenever a smaller value
// arrives.
func TestBoundedSmallest(t *testing.T) {
	const k = 16

	random := rand.New(rand.NewPCG(1, 2))
	stream := make([]uint64, 1000)
	for i := range stream {
		stream[i] = random.Uint64()
	}

	heap := From(make([]uint64, 0, k), Greater[uint64])
	for _, value := range stream {
		switch root, ok := heap.Peek(); {
		case heap.Len() < k:
			heap.Push(value)
		case ok && value < root:
			heap.Replace(value)
		}
		checkInvariant(t, heap)
	}

	want := slices.Clone(stream)
	slices.Sort(want)

	got := drain(heap)
	slices.Sort(got) // A max-heap drains largest first.

	if !slices.Equal(got, want[:k]) {
		t.Errorf("kept %v, want the %d smallest %v", got, k, want[:k])
	}
}

func TestCustomComparator(t *testing.T) {
	type book struct {
		title string
		pages int
	}

	heap := New(func(a, b book) bool { return a.pages < b.pages })
	for _, item := range []book{{"long", 900}, {"short", 40}, {"middling", 300}} {
		heap.Push(item)
	}

	root, ok := heap.Peek()
	if !ok || root.title != "short" {
		t.Errorf("Peek returned %+v, want the shortest book", root)
	}
}

func TestRandomizedAgainstSort(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))

	for range 50 {
		values := make([]int, random.IntN(64))
		for i := range values {
			values[i] = random.IntN(100)
		}

		heap := From(slices.Clone(values), Less[int])
		checkInvariant(t, heap)

		want := slices.Clone(values)
		slices.Sort(want)

		if got := drain(heap); !slices.Equal(got, want) {
			t.Fatalf("drained %v, want %v", got, want)
		}
	}
}

// drain pops the heap empty, returning the elements in the heap's own order.
func drain[T any](heap *Heap[T]) []T {
	drained := make([]T, 0, heap.Len())
	for {
		root, ok := heap.Pop()
		if !ok {
			return drained
		}
		drained = append(drained, root)
	}
}
