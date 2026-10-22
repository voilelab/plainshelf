package sketch

import "github.com/cespare/xxhash/v2"

func eachShingle(s string, n int, fn func(hash uint64)) {
	if n < 1 || s == "" {
		return
	}

	runeStartRing := make([]int, n)
	count := 0

	for offset := range s {
		if count >= n {
			fn(xxhash.Sum64String(s[runeStartRing[(count-n)%n]:offset]))
		}
		runeStartRing[count%n] = offset
		count++
	}

	if count >= n {
		fn(xxhash.Sum64String(s[runeStartRing[(count-n)%n]:]))
	}
}
