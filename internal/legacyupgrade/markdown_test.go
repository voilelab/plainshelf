package legacyupgrade

import (
	"strings"
	"testing"
)

// As in transform_test.go, the expected values come from running the frontend's
// scanMarkdownH2Headings against the same inputs.

func TestScanMarkdownH2Headings(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []MarkdownHeading
	}{
		{
			name:    "no headings",
			content: "just\nsome text",
		},
		{
			name:    "an H3 and horizontal rules are not chapters",
			content: "### three\n---\n***\n## two",
			want:    []MarkdownHeading{{StartOffset: 18, EndOffset: 24, Title: "two"}},
		},
		{
			name:    "a fence only closes with its own marker and run length",
			content: "````md\n~~~\n## still code\n````\n## real",
			want:    []MarkdownHeading{{StartOffset: 30, EndOffset: 37, Title: "real"}},
		},
		{
			name:    "a backtick in the info string means the line is not a fence",
			content: "```md`x\n## inside\n```\nBody",
			want:    []MarkdownHeading{{StartOffset: 8, EndOffset: 17, Title: "inside"}},
		},
		{
			name:    "four leading spaces make indented code, not a heading",
			content: "    ## not a heading\n## yes",
			want:    []MarkdownHeading{{StartOffset: 21, EndOffset: 27, Title: "yes"}},
		},
		{
			name:    "CRLF offsets span the carriage return",
			content: "## One\r\nbody\r\n## Two\r\n",
			want: []MarkdownHeading{
				{StartOffset: 0, EndOffset: 7, Title: "One"},
				{StartOffset: 14, EndOffset: 21, Title: "Two"},
			},
		},
		{
			name:    "only a space or tab may follow the hashes",
			content: "##\u3000第一章\nbody",
		},
		{
			name:    "a title that renders as nothing is named Untitled chapter",
			content: "## **  **\nbody",
			want:    []MarkdownHeading{{StartOffset: 0, EndOffset: 9, Title: "Untitled chapter"}},
		},
		{
			name:    "repeated titles stay separate headings",
			content: "## Same\na\n## Same\nb",
			want: []MarkdownHeading{
				{StartOffset: 0, EndOffset: 7, Title: "Same"},
				{StartOffset: 10, EndOffset: 17, Title: "Same"},
			},
		},
		{
			name:    "inline markup is stripped from the title",
			content: "## `code` and **bold** and *italic*",
			want:    []MarkdownHeading{{StartOffset: 0, EndOffset: 35, Title: "code and bold and italic"}},
		},
		{
			name:    "empty content yields nothing",
			content: "",
		},
		{
			name:    "a heading on the last line without a trailing newline",
			content: "body\n## Last",
			want:    []MarkdownHeading{{StartOffset: 5, EndOffset: 12, Title: "Last"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ScanMarkdownH2Headings(test.content)
			if len(got) != len(test.want) {
				t.Fatalf("got %d headings %+v, want %d", len(got), got, len(test.want))
			}
			for index, heading := range got {
				if heading != test.want[index] {
					t.Errorf("heading %d = %+v, want %+v", index, heading, test.want[index])
				}
			}
		})
	}
}

// TestScanMarkdownH2HeadingsMultiByteOffsets pins the byte-offset contract: a
// heading's range must select exactly its own line even when earlier lines
// contain multi-byte runes.
func TestScanMarkdownH2HeadingsMultiByteOffsets(t *testing.T) {
	content := "一二三\U0001f600\n## 第一章\n內容\n## 第二章"

	headings := ScanMarkdownH2Headings(content)
	if len(headings) != 2 {
		t.Fatalf("got %d headings, want 2", len(headings))
	}

	for _, heading := range headings {
		line := content[heading.StartOffset:heading.EndOffset]
		if !strings.HasPrefix(line, "## ") {
			t.Errorf("range %d:%d selected %q, want the heading line",
				heading.StartOffset, heading.EndOffset, line)
		}
		if strings.ContainsAny(line, "\r\n") {
			t.Errorf("range %d:%d selected %q, want a single line", heading.StartOffset, heading.EndOffset, line)
		}
	}

	if headings[0].Title != "第一章" || headings[1].Title != "第二章" {
		t.Errorf("titles = %q, %q", headings[0].Title, headings[1].Title)
	}
}

func TestTrimJSSpace(t *testing.T) {
	// U+3000 and U+FEFF are trimmed by JavaScript but not by strings.TrimSpace;
	// U+0085 is trimmed by strings.TrimSpace but not by JavaScript.
	if got := trimJSSpace("\u3000\ufeff x \u3000\ufeff"); got != "x" {
		t.Errorf("trimJSSpace() = %q, want %q", got, "x")
	}
	if got := trimJSSpace("\u0085x\u0085"); got != "\u0085x\u0085" {
		t.Errorf("trimJSSpace() = %q, want U+0085 to be kept", got)
	}
}
