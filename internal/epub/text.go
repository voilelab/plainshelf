package epub

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// XHTML inside an EPUB is supposed to be well-formed XML, but real files
// routinely contain undeclared HTML entities and unclosed tags that make a
// strict XML decoder fail on an otherwise readable book. Everything here goes
// through the tolerant HTML parser instead.

// blockTags are elements whose boundaries become line breaks in the flattened
// text. Anything not listed is treated as inline.
var blockTags = map[string]struct{}{
	"address": {}, "article": {}, "aside": {}, "blockquote": {}, "dd": {},
	"div": {}, "dl": {}, "dt": {}, "figcaption": {}, "figure": {}, "footer": {},
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {}, "header": {},
	"hr": {}, "li": {}, "main": {}, "nav": {}, "ol": {}, "p": {}, "pre": {},
	"section": {}, "table": {}, "tr": {}, "ul": {},
}

// skipTags carry no readable prose. rt and rp are ruby annotations: keeping
// them would interleave furigana into the middle of Japanese sentences.
var skipTags = map[string]struct{}{
	"head": {}, "script": {}, "style": {}, "title": {}, "rt": {}, "rp": {},
}

var headingTags = map[string]struct{}{
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
}

// documentToChapter flattens one spine document into a title and body text.
//
// The document's own first heading is consumed as the title and removed from
// the body: every template prints the chapter title itself, so leaving the
// heading in place would print it twice. The caller may still prefer a table of
// contents title over the returned one.
func documentToChapter(data []byte, opts Options) (string, string) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", ""
	}

	root := findElement(doc, "body")
	if root == nil {
		root = doc
	}

	title := ""
	if heading := findFirstHeading(root); heading != nil {
		title = collapseSpaces(textContent(heading))
		if title != "" && heading.Parent != nil {
			heading.Parent.RemoveChild(heading)
		}
	}

	return title, extractText(root, opts)
}

// parseNavDocument reads an EPUB 3 navigation document into href -> title. The
// hrefs are returned exactly as written; the caller resolves them against the
// navigation document's own directory.
func parseNavDocument(data []byte) map[string]string {
	out := make(map[string]string)

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return out
	}

	root := findTOCNav(doc)
	if root == nil {
		root = findElement(doc, "body")
	}
	if root == nil {
		return out
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := strings.TrimSpace(attr(n, "href"))
			title := collapseSpaces(textContent(n))
			if href != "" && title != "" {
				if _, exists := out[href]; !exists {
					out[href] = title
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	return out
}

func extractText(root *html.Node, opts Options) string {
	var b strings.Builder
	writeNode(root, &b, opts)
	return normalizeText(b.String())
}

func writeNode(n *html.Node, b *strings.Builder, opts Options) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(collapseInlineSpaces(n.Data))
		return

	case html.ElementNode:
		if _, skip := skipTags[n.Data]; skip {
			return
		}
		switch n.Data {
		case "br":
			b.WriteByte('\n')
			return
		case "img", "image", "svg":
			// Illustrations are not imported.
			return
		}

		_, isBlock := blockTags[n.Data]
		if isBlock {
			b.WriteByte('\n')
		}

		if marker := emphasisMarker(n.Data, opts); marker != "" {
			writeEmphasis(n, b, opts, marker)
		} else {
			writeChildren(n, b, opts)
		}

		if isBlock {
			b.WriteByte('\n')
		}
		return
	}

	writeChildren(n, b, opts)
}

func writeChildren(n *html.Node, b *strings.Builder, opts Options) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeNode(c, b, opts)
	}
}

func emphasisMarker(tag string, opts Options) string {
	if !opts.MarkdownInline {
		return ""
	}
	switch tag {
	case "em", "i", "cite":
		return "*"
	case "strong", "b":
		return "**"
	default:
		return ""
	}
}

// writeEmphasis wraps inline emphasis in Markdown markers, keeping surrounding
// spaces outside the markers. "* text *" is not emphasis in Markdown, so the
// padding has to move out for the marker to mean anything.
func writeEmphasis(n *html.Node, b *strings.Builder, opts Options, marker string) {
	var inner strings.Builder
	writeChildren(n, &inner, opts)

	raw := inner.String()
	trimmed := strings.Trim(raw, " ")
	if trimmed == "" {
		b.WriteString(raw)
		return
	}

	if strings.HasPrefix(raw, " ") {
		b.WriteByte(' ')
	}
	b.WriteString(marker)
	b.WriteString(trimmed)
	b.WriteString(marker)
	if strings.HasSuffix(raw, " ") {
		b.WriteByte(' ')
	}
}

// collapseInlineSpaces folds runs of HTML whitespace into a single space while
// preserving whether the run touched the start or end of the text node, so that
// "a <em>b</em>" does not become "ab".
//
// Two space-like characters are deliberately excluded and handled later:
// U+3000 IDEOGRAPHIC SPACE, because Chinese novels use it for paragraph
// indentation, and U+00A0 NO-BREAK SPACE, because an author who wrote &nbsp;
// meant a space to be there and dropCJKJoinSpaces must not mistake it for a
// source line wrap.
func collapseInlineSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	pendingSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			pendingSpace = true
		default:
			if pendingSpace {
				b.WriteByte(' ')
				pendingSpace = false
			}
			b.WriteRune(r)
		}
	}
	if pendingSpace {
		b.WriteByte(' ')
	}

	return b.String()
}

// normalizeText trims each line and reduces any run of blank lines to exactly
// one, so paragraphs are separated by a single blank line regardless of how the
// source markup was nested.
//
// Runs of spaces inside a line are also collapsed. Adjacent inline elements each
// contribute their own boundary space ("said " + " softly "), and a reader has
// no way to tell an intentional double space from that artifact.
func normalizeText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	pendingBlank := false
	for _, line := range lines {
		line = normalizeLine(line)
		if line == "" {
			pendingBlank = true
			continue
		}
		if pendingBlank && len(out) > 0 {
			out = append(out, "")
		}
		pendingBlank = false
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// normalizeLine cleans one line of flattened text. The order matters: no-break
// spaces only become ordinary spaces after the CJK line-wrap pass has run, so an
// explicit &nbsp; between two CJK characters survives while a source line wrap
// between the same two characters does not.
func normalizeLine(line string) string {
	line = strings.Trim(line, " \t\r\u00a0")
	line = collapseSpaceRuns(line)
	line = dropCJKJoinSpaces(line)
	line = strings.ReplaceAll(line, "\u00a0", " ")
	return collapseSpaceRuns(line)
}

func collapseSpaces(s string) string {
	return strings.TrimSpace(collapseSpaceRuns(collapseInlineSpaces(s)))
}

func collapseSpaceRuns(s string) string {
	if !strings.Contains(s, "  ") {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	prevSpace := false
	for _, r := range s {
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}

	return b.String()
}

// dropCJKJoinSpaces removes a space that only exists because the source markup
// wrapped a line mid-sentence.
//
// HTML turns a newline inside a paragraph into a space, which is right for
// space-separated scripts and wrong for CJK: "他走出了車站，\n天色早" must join
// as "他走出了車站，天色早", not "…， 天色早". The space is kept whenever either
// side is non-CJK, so "中文 English" and "said softly" are untouched.
func dropCJKJoinSpaces(s string) string {
	if !strings.Contains(s, " ") {
		return s
	}

	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	for i, r := range runes {
		if r == ' ' && i > 0 && i+1 < len(runes) &&
			isCJK(runes[i-1]) && isCJK(runes[i+1]) {
			continue
		}
		out = append(out, r)
	}

	return string(out)
}

// isCJK reports whether r belongs to a script that does not separate words with
// spaces, including the CJK punctuation and fullwidth blocks so that a break
// after "，" or "）" is handled too.
func isCJK(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F: // CJK symbols and punctuation
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // halfwidth and fullwidth forms
		return true
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return true
	default:
		return false
	}
}

func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func findFirstHeading(n *html.Node) *html.Node {
	if n.Type == html.ElementNode {
		if _, ok := headingTags[n.Data]; ok {
			return n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstHeading(c); found != nil {
			return found
		}
	}
	return nil
}

// findTOCNav locates the <nav epub:type="toc"> element. The HTML parser keeps
// the prefixed attribute name intact rather than splitting the namespace, so
// both spellings are checked.
func findTOCNav(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "nav" {
		for _, a := range n.Attr {
			isEpubType := a.Key == "epub:type" || (a.Namespace == "epub" && a.Key == "type")
			if !isEpubType {
				continue
			}
			for v := range strings.FieldsSeq(a.Val) {
				if strings.EqualFold(v, "toc") {
					return n
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findTOCNav(c); found != nil {
			return found
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode {
			if _, skip := skipTags[n.Data]; skip {
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
