package util

import (
	"bufio"
	"io"
	"strings"
)

type ChapterReader struct {
	scanner   *bufio.Scanner
	chapters  []string
	current   int
	lineCount int
}

func NewChapterReader(fp io.Reader, lineCount int) *ChapterReader {
	scanner := bufio.NewScanner(fp)
	scanner.Split(bufio.ScanLines)
	cr := &ChapterReader{
		scanner:   scanner,
		lineCount: lineCount,
	}
	cr.loadChapter()
	return cr
}

func (cr *ChapterReader) loadChapter() bool {
	var chapterLines []string
	for cr.scanner.Scan() {
		chapterLines = append(chapterLines, cr.scanner.Text())
		if len(chapterLines) >= cr.lineCount {
			break
		}
	}

	if len(chapterLines) == 0 {
		return false
	}

	cr.chapters = append(cr.chapters, joinLines(chapterLines))
	return true
}

func (cr *ChapterReader) Index() int {
	return cr.current
}

func (cr *ChapterReader) Prev() {
	if cr.current > 0 {
		cr.current--
	}
}

func (cr *ChapterReader) Next() {
	if cr.current < len(cr.chapters)-1 {
		cr.current++
		return
	}

	if cr.loadChapter() {
		cr.current++
	}
}

func (cr *ChapterReader) HasPrev() bool {
	return cr.current > 0
}

func (cr *ChapterReader) HasNext() bool {
	if cr.current < len(cr.chapters)-1 {
		return true
	}
	return cr.loadChapter()
}

func (cr *ChapterReader) Current() string {
	if cr.current < len(cr.chapters) {
		return cr.chapters[cr.current]
	}
	return ""
}

func joinLines(lines []string) string {
	var strBuilder strings.Builder
	for _, line := range lines {
		strBuilder.WriteString(line)
		strBuilder.WriteString("\n")
	}
	return strBuilder.String()
}
