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

// TextCounter must agree with whole-input counting however the input is split,
// including runes and invalid sequences cut across writes.
func TestTextCounterMatchesWholeInputAcrossSplits(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"\n",
		"line1\nline2",
		"一行文字 line\n第二行",
		"\xe4\xb8",         // truncated rune at EOF
		"\xe4\xb8A\xff\n",  // truncated rune, then ASCII and an invalid byte
		"\xf0\x9f\x98\x80", // 4-byte rune
		"\x80\x80\x80\xe4\xb8\xad",
		strings.Repeat("中a\n", 50) + "\xf0\x9f",
	}
	for _, in := range inputs {
		wantLines := bytes.Count([]byte(in), []byte{'\n'})
		if in != "" && !strings.HasSuffix(in, "\n") {
			wantLines++
		}
		wantChars := scanRuneCount(in)
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
	}
}

// scanRuneCount is the rune count CharCount reported before it streamed.
func scanRuneCount(s string) int {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanRunes)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}
