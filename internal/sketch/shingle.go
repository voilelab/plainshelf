package sketch

import "github.com/cespare/xxhash/v2"

// eachShingle calls fn with the hash of every n-rune window of s, in order and
// including repeats.
//
// Windows are counted in runes rather than bytes so that the shingle width
// means the same thing for CJK text as for alphabetic text, but each window is
// hashed over its bytes, which is equivalent and avoids allocating.
func eachShingle(s string, n int, fn func(hash uint64)) {
	if n < 1 || s == "" {
		return
	}

	// starts is a ring buffer of the byte offsets of the last n runes.
	starts := make([]int, n)
	count := 0

	for offset := range s {
		if count >= n {
			// The window of the n runes before this one ends where it starts.
			fn(xxhash.Sum64String(s[starts[(count-n)%n]:offset]))
		}
		starts[count%n] = offset
		count++
	}

	if count >= n {
		fn(xxhash.Sum64String(s[starts[(count-n)%n]:]))
	}
}
