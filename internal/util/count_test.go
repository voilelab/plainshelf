package util

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestTextCounterLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single line", "hello world", 1},
		{"multiple lines", "line1\nline2\nline3", 3},
		{"trailing newline", "line1\nline2\n", 2},
		{"long line", strings.Repeat("a", 128*1024), 1},
		{"long line with newline", strings.Repeat("a", 128*1024) + "\nshort", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c TextCounter
			_, _ = c.Write([]byte(tt.input))
			if got := c.Lines(); got != tt.want {
				t.Errorf("Lines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTextCounterChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"ASCII", "hello", 5},
		{"Unicode", "你好世界", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c TextCounter
			_, _ = c.Write([]byte(tt.input))
			if got := c.Chars(); got != tt.want {
				t.Errorf("Chars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTextCounterMatchesWholeInputAcrossSplits(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"empty input", ""},
		{"single ASCII rune", "a"},
		{"newline only", "\n"},
		{"two lines without final newline", "line1\nline2"},
		{"mixed CJK and ASCII lines", "一行文字 line\n第二行"},
		{"truncated three-byte rune", "\xe4\xb8"},
		{"truncated rune followed by ASCII and invalid byte", "\xe4\xb8A\xff\n"},
		{"four-byte emoji", "\xf0\x9f\x98\x80"},
		{"invalid continuation bytes before a valid rune", "\x80\x80\x80\xe4\xb8\xad"},
		{"long mixed input ending in a truncated rune", strings.Repeat("中a\n", 50) + "\xf0\x9f"},
	}
	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.input
			wantLines := bytes.Count([]byte(in), []byte{'\n'})
			if in != "" && !strings.HasSuffix(in, "\n") {
				wantLines++
			}
			wantChars := wholeInputRuneCount(in)
			for split := range len(in) + 1 {
				for step := 1; step <= 5; step++ {
					var c TextCounter
					_, _ = c.Write([]byte(in[:split]))
					for i := split; i < len(in); i += step {
						_, _ = c.Write([]byte(in[i:min(i+step, len(in))]))
					}
					if c.Lines() != wantLines || c.Chars() != wantChars {
						t.Fatalf("%q split %d step %d: lines %d chars %d, want %d %d",
							in, split, step, c.Lines(), c.Chars(), wantLines, wantChars)
					}
				}
			}
		})
	}
}

func wholeInputRuneCount(s string) int {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanRunes)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}
