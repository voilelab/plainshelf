package sketch

import "github.com/cespare/xxhash/v2"

func eachShingle(s string, n int, fn func(hash uint64)) {
	if n < 1 || s == "" {
		return
	}

	starts := make([]int, n)
	count := 0

	for offset := range s {
		if count >= n {
			fn(xxhash.Sum64String(s[starts[(count-n)%n]:offset]))
		}
		starts[count%n] = offset
		count++
	}

	if count >= n {
		fn(xxhash.Sum64String(s[starts[(count-n)%n]:]))
	}
}
