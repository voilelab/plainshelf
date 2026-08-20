// Package legacyupgrade upgrades legacy shelf sources - those written before
// source metadata owned the content format - to source metadata schema v1.
//
// It backs a one-off migration tool and is not part of the running server. The
// text transform is a deliberate port of the frontend's upgrade path, so that a
// migrated source reads exactly as the source editor's conversion would have
// produced:
//
//	frontend/src/features/sources/utils/sourceConversions.ts   upgradeLegacyToMarkdown
//	frontend/src/features/reader/utils/markdownChapters.ts     scanMarkdownH2Headings
//	frontend/src/features/reader/utils/markdownLineSyntax.ts     heading and fence syntax
//
// Keep the two in step: a divergence here silently moves chapter boundaries.
package legacyupgrade

import (
	"regexp"
	"strings"
)

// jsSpaceChars is JavaScript's WhiteSpace plus LineTerminator - the set both
// String.prototype.trim and the regexp \s class use. Go differs at both ends:
// strings.TrimSpace adds U+0085 and omits U+00A0, U+FEFF and the Zs block, and
// Go's \s is only [\t\n\f\r ]. The port therefore spells the set out instead of
// inheriting either.
const jsSpaceChars = "\t\n\v\f\r \u00a0\u1680" +
	"\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a" +
	"\u2028\u2029\u202f\u205f\u3000\ufeff"

// jsSpaceClass is jsSpaceChars as a regexp character-class body.
const jsSpaceClass = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// jsDot is JavaScript's `.`, which excludes every LineTerminator. Go's `.` only
// excludes \n, so the port names the class rather than using a bare dot.
const jsDot = `[^\n\r\x{2028}\x{2029}]`

var (
	headingRE      = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(` + jsDot + `+)$`)
	fenceOpenerRE  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(" + jsDot + `*)$`)
	fenceCloserRE  = regexp.MustCompile("^ {0,3}(`+|~+)[ \t]*$")
	inlineCodeRE   = regexp.MustCompile("`([^`]*)`")
	inlineBoldRE   = regexp.MustCompile(`\*\*([^*]*)\*\*`)
	inlineItalicRE = regexp.MustCompile(`\*([^*]*)\*`)

	// leadingHashesRE strips an existing ATX marker before a line is reused as a
	// heading title. This one is ours rather than the user's, so it uses the
	// exact JavaScript \s set.
	leadingHashesRE = regexp.MustCompile(`^#{1,6}[` + jsSpaceClass + `]+`)
)

// trimJSSpace is String.prototype.trim.
func trimJSSpace(value string) string {
	return strings.Trim(value, jsSpaceChars)
}

// MarkdownHeading is one H2 chapter heading line.
//
// StartOffset and EndOffset are byte offsets; the frontend's equivalent are
// UTF-16 code-unit offsets, because there they feed reading progress. Nothing
// in this package slices a string with them - they exist for tests and
// reporting - so the coordinate difference cannot corrupt content.
type MarkdownHeading struct {
	StartOffset int
	EndOffset   int
	Title       string
}

// markdownFence is an open fenced-code block.
type markdownFence struct {
	marker byte
	length int
}

// visibleHeadingText drops the inline markup a heading title renders without.
func visibleHeadingText(value string) string {
	value = inlineCodeRE.ReplaceAllString(value, "${1}")
	value = inlineBoldRE.ReplaceAllString(value, "${1}")
	value = inlineItalicRE.ReplaceAllString(value, "${1}")
	return trimJSSpace(value)
}

// parseMarkdownHeadingLine reports the ATX heading a line is, if any.
func parseMarkdownHeadingLine(rawLine string) (level int, title string, ok bool) {
	match := headingRE.FindStringSubmatch(rawLine)
	if match == nil {
		return 0, "", false
	}
	return len(match[1]), trimJSSpace(match[2]), true
}

// markdownFenceOpener reports the fence a line opens, if any.
func markdownFenceOpener(rawLine string) (markdownFence, bool) {
	match := fenceOpenerRE.FindStringSubmatch(rawLine)
	if match == nil {
		return markdownFence{}, false
	}
	// A backtick fence's info string cannot contain a backtick. Treating such a
	// line as text keeps the scanner aligned with CommonMark and the renderer.
	if match[1][0] == '`' && strings.Contains(match[2], "`") {
		return markdownFence{}, false
	}
	return markdownFence{marker: match[1][0], length: len(match[1])}, true
}

// closesMarkdownFence reports whether a line closes the currently open fence.
func closesMarkdownFence(rawLine string, fence markdownFence) bool {
	match := fenceCloserRE.FindStringSubmatch(rawLine)
	return match != nil && match[1][0] == fence.marker && len(match[1]) >= fence.length
}

// updateMarkdownFenceState advances fenced-code state and reports whether this
// line is the compatible opener or closer. Fence-like text inside a block is
// ordinary code unless it uses the opener's marker with at least its run length.
func updateMarkdownFenceState(rawLine string, current *markdownFence) (state *markdownFence, boundary bool) {
	if current != nil {
		if closesMarkdownFence(rawLine, *current) {
			return nil, true
		}
		return current, false
	}
	if opener, ok := markdownFenceOpener(rawLine); ok {
		return &opener, true
	}
	return nil, false
}

// forEachLine walks content one line at a time, keeping each line's newline so
// the running offset stays exact for CRLF and for a file with no final newline.
func forEachLine(content string, visit func(lineWithNewline string, offset int)) {
	for offset := 0; offset < len(content); {
		index := strings.IndexByte(content[offset:], '\n')
		if index < 0 {
			visit(content[offset:], offset)
			return
		}
		visit(content[offset:offset+index+1], offset)
		offset += index + 1
	}
}

// ScanMarkdownH2Headings returns the H2 chapter headings, excluding any inside
// a fenced code block.
func ScanMarkdownH2Headings(content string) []MarkdownHeading {
	var headings []MarkdownHeading
	var fence *markdownFence

	forEachLine(content, func(lineWithNewline string, offset int) {
		lineWithoutNewline := strings.TrimSuffix(lineWithNewline, "\n")
		rawLine := strings.TrimSuffix(lineWithoutNewline, "\r")

		state, boundary := updateMarkdownFenceState(rawLine, fence)
		if boundary {
			fence = state
			return
		}
		if fence != nil {
			return
		}
		level, title, ok := parseMarkdownHeadingLine(rawLine)
		if !ok || level != 2 {
			return
		}
		visible := visibleHeadingText(title)
		if visible == "" {
			visible = "Untitled chapter"
		}
		headings = append(headings, MarkdownHeading{
			StartOffset: offset,
			EndOffset:   offset + len(lineWithoutNewline),
			Title:       visible,
		})
	})

	return headings
}
