package epub

import (
	"regexp"
	"strings"
	"testing"
)

func sampleBook() *Book {
	return &Book{
		Title:       "星塵集",
		Authors:     []string{"林望舒"},
		Language:    "zh-Hant",
		Description: "一部關於旅途的短篇小說。",
		Chapters: []Chapter{
			{Title: "啟程", Text: "他走出了車站。\n\n風很大。"},
			{Title: "歸途", Text: "回程的路上，他沉默不語。"},
		},
	}
}

func TestRenderMarkdownPreset(t *testing.T) {
	got := Render(sampleBook(), Strategy{Preset: PresetMarkdown, IncludeDescription: true})

	want := strings.Join([]string{
		"# 星塵集",
		"",
		"一部關於旅途的短篇小說。",
		"",
		"## 啟程",
		"",
		"他走出了車站。",
		"",
		"風很大。",
		"",
		"## 歸途",
		"",
		"回程的路上，他沉默不語。",
	}, "\n")

	if got.Text != want {
		t.Errorf("Text =\n%q\nwant\n%q", got.Text, want)
	}
	if got.Format != "md" {
		t.Errorf("Format = %q, want md", got.Format)
	}
	if got.ChapterRegex != "^## " {
		t.Errorf("ChapterRegex = %q, want the heading prefix anchored to a line start", got.ChapterRegex)
	}
}

func TestRenderPlainPreset(t *testing.T) {
	got := Render(sampleBook(), Strategy{Preset: PresetPlain, IncludeDescription: false})

	want := strings.Join([]string{
		"星塵集",
		"",
		"啟程",
		"",
		"他走出了車站。",
		"",
		"風很大。",
		"",
		"歸途",
		"",
		"回程的路上，他沉默不語。",
	}, "\n")

	if got.Text != want {
		t.Errorf("Text =\n%q\nwant\n%q", got.Text, want)
	}
	if got.Format != "txt" {
		t.Errorf("Format = %q, want txt", got.Format)
	}
	// A bare title line has no literal prefix to anchor on, so the reader has to
	// fall back to explicit boundaries.
	if got.ChapterRegex != "" {
		t.Errorf("ChapterRegex = %q, want empty for the plain preset", got.ChapterRegex)
	}
}

// TestRenderChapterLines is the load-bearing test for the reader's chapter list:
// the recorded line numbers must be the exact 1-based lines each chapter starts
// on, because SplitConfig boundaries index into the rendered text.
func TestRenderChapterLines(t *testing.T) {
	tests := []struct {
		name     string
		strategy Strategy
		want     []int
	}{
		{
			name:     "markdown with description",
			strategy: Strategy{Preset: PresetMarkdown, IncludeDescription: true},
			want:     []int{5, 11},
		},
		{
			name:     "markdown without description",
			strategy: Strategy{Preset: PresetMarkdown, IncludeDescription: false},
			want:     []int{3, 9},
		},
		{
			name:     "plain without description",
			strategy: Strategy{Preset: PresetPlain, IncludeDescription: false},
			want:     []int{3, 9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(sampleBook(), tt.strategy)

			if len(got.ChapterLines) != len(tt.want) {
				t.Fatalf("ChapterLines = %v, want %v", got.ChapterLines, tt.want)
			}

			lines := strings.Split(got.Text, "\n")
			for i, lineNo := range got.ChapterLines {
				if lineNo != tt.want[i] {
					t.Errorf("ChapterLines[%d] = %d, want %d", i, lineNo, tt.want[i])
				}
				if lineNo < 1 || lineNo > len(lines) {
					t.Fatalf("ChapterLines[%d] = %d is outside the %d rendered lines", i, lineNo, len(lines))
				}
				// The recorded line must be the chapter's title line.
				if !strings.Contains(lines[lineNo-1], sampleBook().Chapters[i].Title) {
					t.Errorf("line %d is %q, want it to contain chapter title %q",
						lineNo, lines[lineNo-1], sampleBook().Chapters[i].Title)
				}
			}
		})
	}
}

// TestRenderRegexMatchesChapterLines checks the two split mechanisms agree: the
// derived regex must match at exactly the recorded chapter lines.
func TestRenderRegexMatchesChapterLines(t *testing.T) {
	got := Render(sampleBook(), Strategy{Preset: PresetMarkdown, IncludeDescription: true})

	re, err := regexp.Compile("(?m)" + got.ChapterRegex)
	if err != nil {
		t.Fatalf("compile derived regex %q: %v", got.ChapterRegex, err)
	}

	var matchedLines []int
	for lineIdx, line := range strings.Split(got.Text, "\n") {
		if re.MatchString(line) {
			matchedLines = append(matchedLines, lineIdx+1)
		}
	}

	if len(matchedLines) != len(got.ChapterLines) {
		t.Fatalf("regex matched lines %v, want %v", matchedLines, got.ChapterLines)
	}
	for i := range matchedLines {
		if matchedLines[i] != got.ChapterLines[i] {
			t.Errorf("regex matched line %d, want %d", matchedLines[i], got.ChapterLines[i])
		}
	}
}

func TestRenderEmptyFieldsLeaveNoBlankHoles(t *testing.T) {
	book := &Book{
		Title:    "無簡介",
		Chapters: []Chapter{{Title: "", Text: "只有內文。"}},
	}

	got := Render(book, Strategy{Preset: PresetMarkdown, IncludeDescription: true})

	if strings.Contains(got.Text, "\n\n\n") {
		t.Errorf("Text = %q, want no run of blank lines from the empty description", got.Text)
	}
	if strings.HasPrefix(got.Text, "\n") || strings.HasSuffix(got.Text, "\n") {
		t.Errorf("Text = %q, want no leading or trailing blank lines", got.Text)
	}
	if !strings.Contains(got.Text, "只有內文。") {
		t.Errorf("Text = %q, want the chapter body", got.Text)
	}
}

func TestSubstitute(t *testing.T) {
	vars := map[string]string{"book_title": "書名", "empty": ""}

	tests := []struct {
		tmpl string
		want string
	}{
		{"{book_title}", "書名"},
		{"# {book_title} #", "# 書名 #"},
		{"{empty}", ""},
		{"{{book_title}}", "{book_title}}"},
		{"{{}", "{}"},
		{"{unknown}", "{unknown}"},
		{"no placeholders", "no placeholders"},
		{"{unterminated", "{unterminated"},
	}

	for _, tt := range tests {
		if got := substitute(tt.tmpl, vars); got != tt.want {
			t.Errorf("substitute(%q) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}

func TestStrategyValidate(t *testing.T) {
	tests := []struct {
		preset  Preset
		wantErr bool
	}{
		{PresetMarkdown, false},
		{PresetPlain, false},
		{"", true},
		{"custom", true},
		{"MARKDOWN", true},
	}

	for _, tt := range tests {
		err := Strategy{Preset: tt.preset}.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Strategy{Preset: %q}.Validate() error = %v, wantErr %v", tt.preset, err, tt.wantErr)
		}
	}
}

func TestStrategyDerivedValues(t *testing.T) {
	tests := []struct {
		name           string
		strategy       Strategy
		wantFormat     string
		wantMarkdownIn bool
	}{
		{"markdown", Strategy{Preset: PresetMarkdown}, "md", true},
		{"plain", Strategy{Preset: PresetPlain}, "txt", false},
		{"empty falls back to the default preset", Strategy{}, "md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.Format(); got != tt.wantFormat {
				t.Errorf("Format() = %q, want %q", got, tt.wantFormat)
			}
			if got := tt.strategy.ParseOptions().MarkdownInline; got != tt.wantMarkdownIn {
				t.Errorf("ParseOptions().MarkdownInline = %v, want %v", got, tt.wantMarkdownIn)
			}
		})
	}
}
