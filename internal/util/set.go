package util

type Set[T comparable] struct {
	m map[T]bool
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]bool)}
}

func (s *Set[T]) Add(item T) {
	s.m[item] = true
}

func (s *Set[T]) Remove(item T) {
	delete(s.m, item)
}

func (s *Set[T]) Contains(item T) bool {
	_, exists := s.m[item]
	return exists
}

func (s *Set[T]) Items() []T {
	items := make([]T, 0, len(s.m))
	for item := range s.m {
		items = append(items, item)
	}
	return items
}

func (s *Set[T]) Copy() *Set[T] {
	newSet := NewSet[T]()
	for item := range s.m {
		newSet.Add(item)
	}
	return newSet
}
