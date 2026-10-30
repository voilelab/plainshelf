package sketch

import (
	"slices"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/container"
)

const (
	DefaultN = 5

	DefaultK = 128

	ExactShingleLimit = 5000
)

type Sketch struct {
	N        int
	Distinct int
	Values   []uint64
}

func (s Sketch) complete() bool {
	return len(s.Values) == s.Distinct
}

func BuildDefault(normalized string) Sketch {
	k := DefaultK
	if size := utf8.RuneCountInString(normalized); size < ExactShingleLimit {
		keepAllShingles := size + 1
		k = keepAllShingles
	}

	return Build(normalized, DefaultN, k)
}

func Build(normalized string, n, k int) Sketch {
	if n < 1 {
		n = DefaultN
	}
	if k < 1 {
		k = DefaultK
	}

	const maxInitialHeapCap = 1024

	seen := map[uint64]struct{}{}
	bottom := container.HeapFrom(make([]uint64, 0, min(k, maxInitialHeapCap)), container.Greater[uint64])

	eachShingle(normalized, n, func(hash uint64) {
		if _, duplicate := seen[hash]; duplicate {
			return
		}
		seen[hash] = struct{}{}

		if bottom.Len() < k {
			bottom.Push(hash)

			return
		}
		if largest, _ := bottom.Peek(); hash < largest {
			bottom.Replace(hash)
		}
	})

	values := bottom.Items()
	slices.Sort(values)

	return Sketch{N: n, Distinct: len(seen), Values: values}
}
