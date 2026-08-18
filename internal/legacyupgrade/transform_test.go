package legacyupgrade

import (
	"errors"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// The expected values below were produced by running the frontend's
// upgradeLegacyToMarkdown against the same inputs, so they record what the
// source editor actually does rather than what the code appears to do. Two
// cases deliberately diverge; each says so.

func TestUpgradeLegacyToMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		config  shelf.SplitConfig
		want    string
	}{
		{
			name:    "boundary splits at the given line numbers",
			content: "a\nb\nc",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{1, 3}},
			want:    "## Part 1\n\na\nb\n## Part 2\n\nc",
		},
		{
			name:    "existing H2 wins over any split configuration",
			content: "## Existing\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 1},
			want:    "## Existing\nText",
		},
		{
			name:    "regex turns matched lines into H2 titles",
			content: "Chapter 1\nText\nChapter 2",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^Chapter \d+$`},
			want:    "## Chapter 1\nText\n## Chapter 2",
		},
		{
			name:    "an H2 inside a fence is not existing structure",
			content: "```md\n## example only\n```\nBody",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeNone},
			want:    "## Part 1\n\n```md\n## example only\n```\nBody",
		},
		{
			name:    "text above the first match becomes Part 1",
			content: "Intro\nChapter 1\nText\nChapter 2",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^Chapter \d+$`},
			want:    "## Part 1\n\nIntro\n## Chapter 1\nText\n## Chapter 2",
		},
		{
			name:    "a match inside a line promotes the whole line",
			content: "aaa Chapter 1 bbb\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `Chapter \d+`},
			want:    "## aaa Chapter 1 bbb\nText",
		},
		{
			name:    "a CRLF heading line keeps its carriage return",
			content: "Intro\r\nChapter 1\r\nText\r\n",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^Chapter \d+$`},
			want:    "## Part 1\n\nIntro\r\n## Chapter 1\r\nText\r\n",
		},
		{
			name:    "a dot-matching pattern still anchors on a CRLF line",
			content: "Intro\r\n第一章\r\nText\r\n",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^第.*章$`},
			want:    "## Part 1\n\nIntro\r\n## 第一章\r\nText\r\n",
		},
		{
			name:    "two matches on one line produce one heading",
			content: "Chapter 1 Chapter 2\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `Chapter \d+`},
			want:    "## Chapter 1 Chapter 2\nText",
		},
		{
			name:    "a blank regex falls back to a single part",
			content: "a\nb",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: "   "},
			want:    "## Part 1\n\na\nb",
		},
		{
			name:    "a heading line with no title text is named Untitled chapter",
			content: "### \nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^#{1,6}`},
			want:    "## Untitled chapter\nText",
		},
		{
			name:    "an existing ATX marker is stripped from the title",
			content: "### Title\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^###`},
			want:    "## Title\nText",
		},
		{
			// Go's \s is only [\t\n\f\r ]; JavaScript's covers the ideographic
			// space. Without the translation this pattern would match nothing.
			name:    "a pattern using backslash-s matches a full-width space",
			content: "第一章　\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^第.+章\s*$`},
			want:    "## 第一章\nText",
		},
		{
			name:    "out-of-range boundaries are dropped and offset zero remains",
			content: "a\nb\nc",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{0, 1, 99, -3}},
			want:    "## Part 1\n\na\nb\nc",
		},
		{
			name:    "repeated boundaries collapse and are numbered in reading order",
			content: "a\nb\nc",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{1, 1, 2}},
			want:    "## Part 1\n\na\n## Part 2\n\nb\nc",
		},
		{
			name:    "a boundary on the empty line after a trailing newline is kept",
			content: "a\nb\n",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{3}},
			want:    "## Part 1\n\na\nb\n## Part 2\n\n",
		},
		{
			name:    "line_count steps the line table",
			content: "a\nb\nc\nd",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 2},
			want:    "## Part 1\n\na\nb\n## Part 2\n\nc\nd",
		},
		{
			name:    "a non-positive line_count falls back to a single part",
			content: "a\nb",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 0},
			want:    "## Part 1\n\na\nb",
		},
		{
			name:    "line_count counts the empty line after a trailing newline",
			content: "a\nb\n",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 1},
			want:    "## Part 1\n\na\n## Part 2\n\nb\n## Part 3\n\n",
		},
		{
			name:    "empty content does not panic",
			content: "",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 10},
			want:    "## Part 1\n\n",
		},
		{
			name:    "multi-byte content keeps its line boundaries",
			content: "一二三\n😀四\n五",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{2, 3}},
			want:    "## Part 1\n\n一二三\n## Part 2\n\n😀四\n## Part 3\n\n五",
		},
		{
			name:    "line_count preserves CRLF line endings",
			content: "a\r\nb\r\nc",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 2},
			want:    "## Part 1\n\na\r\nb\r\n## Part 2\n\nc",
		},
		{
			// Go has accepted JavaScript's (?<name>…) spelling since 1.22, so a
			// pattern using it needs no translation.
			name:    "a JavaScript named group is accepted as written",
			content: "Chapter 1\nText",
			config:  shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: `^(?<title>Chapter \d+)$`},
			want:    "## Chapter 1\nText",
		},
		{
			name:    "an unrecognized split type falls back to a single part",
			content: "a\nb",
			config:  shelf.SplitConfig{Type: shelf.SplitType("nonsense")},
			want:    "## Part 1\n\na\nb",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := UpgradeLegacyToMarkdown(test.content, test.config)
			if err != nil {
				t.Fatalf("UpgradeLegacyToMarkdown returned error: %v", err)
			}
			if got != test.want {
				t.Errorf("UpgradeLegacyToMarkdown() =\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

func TestUpgradeLegacyToMarkdownRejectsUnusableRegex(t *testing.T) {
	tests := []struct {
		name    string
		content string
		regex   string
		wantErr error
	}{
		{"lookahead", "第一章\nText", `^(?=第)第.+章$`, ErrUnsupportedSplitRegex},
		{"lookbehind", "aChapter\nText", `(?<=a)Chapter`, ErrUnsupportedSplitRegex},
		{"backreference", "aa\nText", `(a)\1`, ErrUnsupportedSplitRegex},
		{
			// The frontend degrades to one chapter here. In a one-off in-place
			// migration that would silently discard the book's structure, so the
			// source is reported and left legacy instead.
			name: "compiles but matches nothing", content: "a\nb", regex: "^ZZZ$",
			wantErr: ErrSplitRegexNoMatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := UpgradeLegacyToMarkdown(test.content,
				shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: test.regex})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if got != "" {
				t.Errorf("content = %q, want no partial output", got)
			}
		})
	}
}

func TestTranslateJSSlashS(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		subject string
		want    bool
	}{
		{"bare backslash-s matches an ideographic space", `^a\s+b$`, "a　b", true},
		{"backslash-s inside a class", `^a[\sx]b$`, "a b", true},
		{"negated backslash-S", `^a\Sb$`, "axb", true},
		{"negated backslash-S rejects a full-width space", `^a\Sb$`, "a　b", false},
		{"an escaped backslash before s is left alone", `^a\\sb$`, `a\sb`, true},
		{"other escapes are untouched", `^\d+$`, "42", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ranges, err := regexLineRanges(test.subject, test.pattern)
			switch {
			case test.want && err != nil:
				t.Fatalf("regexLineRanges(%q, %q) returned error: %v", test.subject, test.pattern, err)
			case test.want && len(ranges) != 1:
				t.Errorf("got %d matched lines, want 1", len(ranges))
			case !test.want && !errors.Is(err, ErrSplitRegexNoMatch):
				t.Errorf("error = %v, want ErrSplitRegexNoMatch", err)
			}
		})
	}
}

func TestPlanUpgrade(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		split        shelf.SplitConfig
		format       string
		wantFormat   string
		wantRewrite  bool
		wantChapters int
		wantReason   bool
	}{
		{
			name: "no split leaves plain text alone", content: "a\nb",
			split: shelf.SplitConfig{}, format: shelf.BookFormatText,
			wantFormat: shelf.BookFormatText, wantChapters: 1,
		},
		{
			name: "no split leaves markdown alone", content: "## One\na\n## Two\nb",
			split: shelf.SplitConfig{}, format: shelf.BookFormatMarkdown,
			wantFormat: shelf.BookFormatMarkdown, wantChapters: 2,
		},
		{
			name: "markdown with no headings still counts one chapter", content: "a\nb",
			split: shelf.SplitConfig{}, format: shelf.BookFormatMarkdown,
			wantFormat: shelf.BookFormatMarkdown, wantChapters: 1,
		},
		{
			name:       "a split over existing headings only claims the format",
			content:    "## One\na\n## Two\nb",
			split:      shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 1},
			format:     shelf.BookFormatText,
			wantFormat: shelf.BookFormatMarkdown, wantChapters: 2, wantReason: true,
		},
		{
			name: "a zero line count names no boundary", content: "a\nb",
			split: shelf.SplitConfig{Type: shelf.SplitTypeLineCount}, format: shelf.BookFormatText,
			wantFormat: shelf.BookFormatText, wantChapters: 1, wantReason: true,
		},
		{
			name: "an empty boundary list names no boundary", content: "a\nb",
			split: shelf.SplitConfig{Type: shelf.SplitTypeBoundary}, format: shelf.BookFormatText,
			wantFormat: shelf.BookFormatText, wantChapters: 1, wantReason: true,
		},
		{
			name: "a blank regex names no boundary", content: "a\nb",
			split: shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: "  "}, format: shelf.BookFormatText,
			wantFormat: shelf.BookFormatText, wantChapters: 1, wantReason: true,
		},
		{
			name: "a split with no headings rewrites the content", content: "a\nb\nc\nd",
			split:      shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 2},
			format:     shelf.BookFormatText,
			wantFormat: shelf.BookFormatMarkdown, wantRewrite: true, wantChapters: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanUpgrade(test.content, test.split, test.format)
			if err != nil {
				t.Fatalf("PlanUpgrade returned error: %v", err)
			}
			if plan.Format != test.wantFormat {
				t.Errorf("Format = %q, want %q", plan.Format, test.wantFormat)
			}
			if plan.Rewrite != test.wantRewrite {
				t.Errorf("Rewrite = %v, want %v", plan.Rewrite, test.wantRewrite)
			}
			if plan.Chapters != test.wantChapters {
				t.Errorf("Chapters = %d, want %d", plan.Chapters, test.wantChapters)
			}
			if (plan.Reason != "") != test.wantReason {
				t.Errorf("Reason = %q, want a reason: %v", plan.Reason, test.wantReason)
			}
			if !plan.Rewrite && plan.Content != "" {
				t.Errorf("Content = %q, want empty when no rewrite is needed", plan.Content)
			}
		})
	}
}

func TestPlanUpgradeRejectsUnknownSplitType(t *testing.T) {
	_, err := PlanUpgrade("a\nb", shelf.SplitConfig{Type: shelf.SplitType("nonsense")}, shelf.BookFormatText)
	if !errors.Is(err, ErrUnrecognizedSplitType) {
		t.Fatalf("error = %v, want ErrUnrecognizedSplitType", err)
	}
	if !strings.Contains(err.Error(), "nonsense") {
		t.Errorf("error %q does not name the offending type", err)
	}
}
