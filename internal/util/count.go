package util

import (
	"bytes"
	"unicode/utf8"
)

type TextCounter struct {
	lines    int
	chars    int
	lineOpen bool
	partial  []byte
}

func (c *TextCounter) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}
	c.lines += bytes.Count(p, []byte{'\n'})
	c.lineOpen = p[n-1] != '\n'

	if len(c.partial) > 0 {
		head := append(c.partial, p[:min(n, utf8.UTFMax)]...)
		i := 0
		for i < len(c.partial) {
			if !utf8.FullRune(head[i:]) {
				c.partial = append([]byte(nil), head[i:]...)
				return n, nil
			}
			_, size := utf8.DecodeRune(head[i:])
			c.chars++
			i += size
		}
		p = p[i-len(c.partial):]
		c.partial = nil
	}

	for k := 1; k <= min(len(p), utf8.UTFMax-1); k++ {
		if tail := p[len(p)-k:]; utf8.RuneStart(tail[0]) {
			if !utf8.FullRune(tail) {
				c.partial = append([]byte(nil), tail...)
				p = p[:len(p)-k]
			}
			break
		}
	}
	c.chars += utf8.RuneCount(p)
	return n, nil
}

func (c *TextCounter) Lines() int {
	if c.lineOpen {
		return c.lines + 1
	}
	return c.lines
}

func (c *TextCounter) Chars() int {
	return c.chars + utf8.RuneCount(c.partial)
}
