package epub

import (
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
}

func TestRenderUntitledChapterGetsStableHeading(t *testing.T) {
	book := sampleBook()
	book.Chapters[1].Title = ""

	got := Render(book, Strategy{Preset: PresetMarkdown})

	if !strings.Contains(got.Text, "## Part 2") {
		t.Errorf("Text = %q, want a stable H2 for the untitled chapter", got.Text)
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
	if !strings.Contains(got.Text, "## Part 1") {
		t.Errorf("Text = %q, want a stable H2 for the untitled chapter", got.Text)
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
	keep, drop := true, false

	tests := []struct {
		name           string
		strategy       Strategy
		wantFormat     string
		wantMarkdownIn bool
		wantKeepImages bool
	}{
		{"markdown", Strategy{Preset: PresetMarkdown}, "md", true, true},
		{"plain", Strategy{Preset: PresetPlain}, "txt", false, false},
		{"empty falls back to the default preset", Strategy{}, "md", true, true},
		// A plain-text book has no image syntax, so the strategy must not ask
		// for illustrations even when the setting says to keep them.
		{"plain never keeps images", Strategy{Preset: PresetPlain, KeepImages: &keep}, "txt", false, false},
		{"markdown honours an explicit keep", Strategy{Preset: PresetMarkdown, KeepImages: &keep}, "md", true, true},
		// A strategy stored before keep_images existed must keep illustrations
		// rather than inherit a false zero value.
		{"markdown honours an explicit drop", Strategy{Preset: PresetMarkdown, KeepImages: &drop}, "md", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.Format(); got != tt.wantFormat {
				t.Errorf("Format() = %q, want %q", got, tt.wantFormat)
			}
			opts := tt.strategy.ParseOptions()
			if opts.MarkdownInline != tt.wantMarkdownIn {
				t.Errorf("ParseOptions().MarkdownInline = %v, want %v", opts.MarkdownInline, tt.wantMarkdownIn)
			}
			if opts.KeepImages != tt.wantKeepImages {
				t.Errorf("ParseOptions().KeepImages = %v, want %v", opts.KeepImages, tt.wantKeepImages)
			}
		})
	}
}
