// Package container holds generic containers the standard library does not.
//
// container/heap predates generics: it drives the heap through an interface
// whose Push and Pop take any, so every element is boxed and each comparison is
// a pair of interface calls. [Heap] is the same algorithm over a type
// parameter, which costs one indirect call per comparison and no allocation per
// element.
package container

import "cmp"

// Heap is a binary heap ordered by less, with the smallest element at the root.
// The zero value is not usable; build one with [New] or [From].
type Heap[T any] struct {
	items []T
	less  func(a, b T) bool
}

// Less reports whether a sorts before b. It is the comparator for a min-heap.
func Less[T cmp.Ordered](a, b T) bool { return a < b }

// Greater reports whether a sorts after b. It is the comparator for a max-heap,
// whose root is the element to evict when the heap holds the smallest values
// seen so far.
func Greater[T cmp.Ordered](a, b T) bool { return a > b }

// New returns an empty heap ordered by less, which must not be nil.
func New[T any](less func(a, b T) bool) *Heap[T] {
	return &Heap[T]{less: less}
}

// From returns a heap over items, reordering them in place in O(len(items)),
// which is cheaper than pushing them one at a time. The caller must not use
// items afterwards. Passing an empty slice with a capacity preallocates.
func From[T any](items []T, less func(a, b T) bool) *Heap[T] {
	heap := &Heap[T]{items: items, less: less}
	for i := len(items)/2 - 1; i >= 0; i-- {
		heap.down(i)
	}

	return heap
}

// Len returns the number of elements in the heap.
func (h *Heap[T]) Len() int { return len(h.items) }

// Items returns the backing slice in heap order, which is not sorted. It is
// valid until the next call that changes the heap.
func (h *Heap[T]) Items() []T { return h.items }

// Peek returns the root without removing it, and false if the heap is empty.
func (h *Heap[T]) Peek() (T, bool) {
	if len(h.items) == 0 {
		var zero T

		return zero, false
	}

	return h.items[0], true
}

// Push adds v in O(log n).
func (h *Heap[T]) Push(v T) {
	h.items = append(h.items, v)
	h.up(len(h.items) - 1)
}

// Pop removes and returns the root in O(log n), and false if the heap is empty.
func (h *Heap[T]) Pop() (T, bool) {
	root, ok := h.Peek()
	if !ok {
		return root, false
	}

	last := len(h.items) - 1
	h.items[0] = h.items[last]
	var zero T
	h.items[last] = zero // Drop the duplicated reference so it can be collected.
	h.items = h.items[:last]
	h.down(0)

	return root, true
}

// Replace overwrites the root with v and returns the displaced root, in one
// sift rather than the two a Pop and a Push would cost. On an empty heap it
// pushes v and reports false, so a bounded top-k loop needs no special case.
func (h *Heap[T]) Replace(v T) (T, bool) {
	root, ok := h.Peek()
	if !ok {
		h.Push(v)

		return root, false
	}

	h.items[0] = v
	h.down(0)

	return root, true
}

// up restores the heap property after the element at i grew smaller, or was
// appended at the end.
func (h *Heap[T]) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !h.less(h.items[i], h.items[parent]) {
			return
		}
		h.items[parent], h.items[i] = h.items[i], h.items[parent]
		i = parent
	}
}

// down restores the heap property after the element at i grew larger, or was
// replaced.
func (h *Heap[T]) down(i int) {
	for {
		smallest := i
		if left := 2*i + 1; left < len(h.items) && h.less(h.items[left], h.items[smallest]) {
			smallest = left
		}
		if right := 2*i + 2; right < len(h.items) && h.less(h.items[right], h.items[smallest]) {
			smallest = right
		}
		if smallest == i {
			return
		}
		h.items[smallest], h.items[i] = h.items[i], h.items[smallest]
		i = smallest
	}
}
