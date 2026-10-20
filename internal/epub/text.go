package epub

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

var blockTags = map[string]struct{}{
	"address": {}, "article": {}, "aside": {}, "blockquote": {}, "dd": {},
	"div": {}, "dl": {}, "dt": {}, "figcaption": {}, "figure": {}, "footer": {},
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {}, "header": {},
	"hr": {}, "li": {}, "main": {}, "nav": {}, "ol": {}, "p": {}, "pre": {},
	"section": {}, "table": {}, "tr": {}, "ul": {},
}

var skipTags = map[string]struct{}{
	"head": {}, "script": {}, "style": {}, "title": {}, "rt": {}, "rp": {},
}

var headingTags = map[string]struct{}{
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
}

func documentToChapter(data []byte, opts Options) (title, text string, imageHrefs []string) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", "", nil
	}

	root := findElement(doc, "body")
	if root == nil {
		root = doc
	}

	imageHrefs = collectImageHrefs(root)

	if heading := findFirstHeading(root); heading != nil {
		title = collapseSpaces(textContent(heading))
		if title != "" && heading.Parent != nil {
			heading.Parent.RemoveChild(heading)
		}
	}

	return title, extractText(root, opts), imageHrefs
}

// <img> carries src; SVG's <image> carries href, or xlink:href in older files.
// The tolerant HTML parser stores the latter with the namespace split off, so
// looking up "href" finds both.
func collectImageHrefs(root *html.Node) []string {
	var hrefs []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var href string
			switch n.Data {
			case "img":
				href = attr(n, "src")
			case "image":
				href = attr(n, "href")
			}
			if href = strings.TrimSpace(href); href != "" {
				hrefs = append(hrefs, href)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	return hrefs
}

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

func sanitizeImageAlt(alt string) string {
	return collapseSpaces(strings.NewReplacer("[", "", "]", "", "\n", " ", "\r", " ").Replace(alt))
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
		case "img":
			if opts.imageTarget != nil {
				if target := opts.imageTarget(attr(n, "src")); target != "" {
					b.WriteByte('\n')
					b.WriteString("![" + sanitizeImageAlt(attr(n, "alt")) + "](" + target + ")")
					b.WriteByte('\n')
				}
			}
			return
		case "image", "svg":
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

func isCJK(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F:
		return true
	case r >= 0xFF00 && r <= 0xFFEF:
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
