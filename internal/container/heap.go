package container

import "cmp"

type Heap[T any] struct {
	items []T
	less  func(a, b T) bool
}

func Less[T cmp.Ordered](a, b T) bool { return a < b }

func Greater[T cmp.Ordered](a, b T) bool { return a > b }

func NewHeap[T any](less func(a, b T) bool) *Heap[T] {
	return &Heap[T]{less: less}
}

func HeapFrom[T any](items []T, less func(a, b T) bool) *Heap[T] {
	heap := &Heap[T]{items: items, less: less}
	for i := len(items)/2 - 1; i >= 0; i-- {
		heap.down(i)
	}

	return heap
}

func (h *Heap[T]) Len() int { return len(h.items) }

func (h *Heap[T]) Items() []T { return h.items }

func (h *Heap[T]) Peek() (T, bool) {
	if len(h.items) == 0 {
		var zero T

		return zero, false
	}

	return h.items[0], true
}

func (h *Heap[T]) Push(v T) {
	h.items = append(h.items, v)
	h.up(len(h.items) - 1)
}

func (h *Heap[T]) Pop() (T, bool) {
	root, ok := h.Peek()
	if !ok {
		return root, false
	}

	last := len(h.items) - 1
	h.items[0] = h.items[last]
	var zero T
	h.items[last] = zero
	h.items = h.items[:last]
	h.down(0)

	return root, true
}

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
