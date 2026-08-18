package legacyupgrade

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

var (
	// ErrUnsupportedSplitRegex reports a legacy split regex Go's RE2 engine
	// cannot run. The frontend degrades to a single chapter when a pattern
	// throws; a one-off in-place migration must not, because that silently
	// discards a book's chapter structure with no way back. The source is
	// reported and left legacy instead.
	ErrUnsupportedSplitRegex = util.NewError("legacy split regex is not supported by Go's regexp engine")

	// ErrSplitRegexNoMatch reports a split regex that compiled but matched
	// nothing. The reader may well have been showing chapters this build cannot
	// reproduce - the JavaScript and RE2 dialects are not identical - and there
	// is no way to tell that apart from a pattern that never matched anything.
	// Reporting the source beats silently migrating it down to one chapter.
	ErrSplitRegexNoMatch = util.NewError("legacy split regex matched no line")

	// ErrUnrecognizedSplitType reports a split type this build does not know.
	ErrUnrecognizedSplitType = util.NewError("unrecognized legacy split type")
)

// Every offset this file computes is 0, len(content), the byte index of a '\n',
// or one past it. A '\n' is ASCII, so those positions denote the same place in
// the text whether they are counted in bytes or in the UTF-16 code units the
// frontend uses, and none of them can land inside a multi-byte rune. The one
// integer that comes from persisted user data - SplitConfig.Boundaries - indexes
// the line table, never the text, so it selects the same line under either
// coordinate system.

// lineStartOffsets returns the offset of every line start, including the empty
// line after a trailing newline.
func lineStartOffsets(content string) []int {
	starts := []int{0}
	for index := range len(content) {
		if content[index] == '\n' {
			starts = append(starts, index+1)
		}
	}
	return starts
}

// lineRange is one whole line that a split regex matched.
type lineRange struct {
	start int
	end   int // exclusive, before the line's newline
}

// translateJSSlashS rewrites JavaScript's \s and \S into the explicit classes
// RE2 needs.
//
// JavaScript's \s is WhiteSpace plus LineTerminator - it covers U+00A0, U+FEFF
// and the whole Zs block - while Go's is only [\t\n\f\r ]. Left untranslated, a
// pattern as ordinary as ^第.+章\s*$ stops matching a line padded with an
// ideographic space, the regex finds nothing, and the book's chapters vanish
// without a word. \S is only translated outside a character class, where a
// negated class is expressible.
func translateJSSlashS(pattern string) string {
	var out strings.Builder
	inClass := false

	for index := 0; index < len(pattern); index++ {
		char := pattern[index]

		if char == '\\' && index+1 < len(pattern) {
			switch escaped := pattern[index+1]; {
			case escaped == 's':
				if inClass {
					out.WriteString(jsSpaceClass)
				} else {
					out.WriteString("[" + jsSpaceClass + "]")
				}
			case escaped == 'S' && !inClass:
				out.WriteString("[^" + jsSpaceClass + "]")
			default:
				out.WriteByte(char)
				out.WriteByte(escaped)
			}
			index++
			continue
		}

		switch {
		case char == '[' && !inClass:
			inClass = true
		case char == ']' && inClass:
			inClass = false
		}
		out.WriteByte(char)
	}

	return out.String()
}

// lineIndexOf reports which line an offset falls on, given a line-start table.
func lineIndexOf(starts []int, offset int) int {
	index, found := slices.BinarySearch(starts, offset)
	if found {
		return index
	}
	return index - 1
}

// regexLineRanges snaps every match to its own line and drops the repeats a
// second match on an already-matched line would produce.
//
// Matching runs against a CR-normalized copy of the content. JavaScript's
// multiline ^ and $ treat a CR as a line terminator and Go's do not, so on a
// CRLF file a pattern like ^Chapter \d+$ matches in the browser and matches
// nothing here - which would quietly collapse every chapter into one. Removing
// only the CR of a CRLF pair leaves the number and order of lines untouched, so
// a match's line index maps straight back onto the original text.
func regexLineRanges(content, pattern string) ([]lineRange, error) {
	re, err := regexp.Compile("(?m)" + translateJSSlashS(pattern))
	if err != nil {
		return nil, util.Errorf("%w: %w", ErrUnsupportedSplitRegex, err)
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalizedStarts := lineStartOffsets(normalized)
	starts := lineStartOffsets(content)

	var ranges []lineRange
	for _, match := range re.FindAllStringIndex(normalized, -1) {
		line := lineIndexOf(normalizedStarts, match[0])

		start := starts[line]
		end := len(content)
		if line+1 < len(starts) {
			end = starts[line+1] - 1 // the line's newline is not part of it
		}

		if len(ranges) > 0 && ranges[len(ranges)-1].start == start {
			continue
		}
		ranges = append(ranges, lineRange{start: start, end: end})
	}

	if len(ranges) == 0 {
		return nil, util.Errorf("%w: %q", ErrSplitRegexNoMatch, pattern)
	}

	return ranges, nil
}

// UpgradeLegacyToMarkdown bakes a legacy split configuration into the text as
// H2 headings, the chapter model schema-v1 Markdown sources use.
//
// Content that already carries H2 headings expresses the intended structure and
// is returned untouched, whatever the split configuration says.
func UpgradeLegacyToMarkdown(content string, config shelf.SplitConfig) (string, error) {
	if len(ScanMarkdownH2Headings(content)) > 0 {
		return content, nil
	}

	var offsets []int

	switch {
	case config.Type == shelf.SplitTypeRegex && trimJSSpace(config.Regex) != "":
		ranges, err := regexLineRanges(content, config.Regex)
		if err != nil {
			return "", util.Errorf("%w", err)
		}

		// Rewrite back to front so the offsets taken from content stay valid,
		// and read every title from content rather than from the accumulator.
		upgraded := content
		for index := len(ranges) - 1; index >= 0; index-- {
			line := ranges[index]

			title := strings.TrimSuffix(content[line.start:line.end], "\r")
			title = trimJSSpace(leadingHashesRE.ReplaceAllString(title, ""))
			if title == "" {
				title = "Untitled chapter"
			}

			carriageReturn := ""
			if line.end > 0 && content[line.end-1] == '\r' {
				carriageReturn = "\r"
			}

			upgraded = upgraded[:line.start] + "## " + title + carriageReturn + upgraded[line.end:]
		}
		if ranges[0].start > 0 {
			// Text above the first chapter needs a chapter of its own. The
			// frontend writes a plain LF here even in a CRLF file; keep that, so
			// the two paths cannot disagree about a migrated source's bytes.
			upgraded = "## Part 1\n\n" + upgraded
		}
		return upgraded, nil

	case config.Type == shelf.SplitTypeBoundary:
		starts := lineStartOffsets(content)
		// Offset 0 is always a boundary: the text above the first configured
		// line is its own chapter.
		offsets = append(offsets, 0)
		for _, line := range config.Boundaries {
			if index := line - 1; index >= 0 && index < len(starts) {
				offsets = append(offsets, starts[index])
			}
		}

	case config.Type == shelf.SplitTypeLineCount && config.LineCount > 0:
		starts := lineStartOffsets(content)
		for index := 0; index < len(starts); index += config.LineCount {
			offsets = append(offsets, starts[index])
		}
	}

	if len(offsets) == 0 {
		return "## Part 1\n\n" + content, nil
	}

	slices.Sort(offsets)
	offsets = slices.Compact(offsets)

	upgraded := content
	for index := len(offsets) - 1; index >= 0; index-- {
		upgraded = upgraded[:offsets[index]] +
			fmt.Sprintf("## Part %d\n\n", index+1) +
			upgraded[offsets[index]:]
	}
	return upgraded, nil
}

// Plan is what one legacy source needs to become a schema-v1 source.
type Plan struct {
	// Format is the format to write into meta.json: "txt" or "md".
	Format string
	// Content is the replacement text, empty when Rewrite is false.
	Content string
	// Rewrite reports whether source.txt has to change.
	Rewrite bool
	// Chapters is how many chapters the migrated source will have.
	Chapters int
	// Reason explains a non-obvious outcome, for the migration report.
	Reason string
}

// SplitProducesChapters reports whether a split configuration can express a
// chapter boundary at all.
//
// A line count of zero, an empty boundary list, or a blank pattern names no
// boundary, so the source already reads as a single chapter. Rewriting it would
// only prepend a "## Part 1" line it never had — and turn plain text into
// Markdown — for no gain. This is not hypothetical: shelf.SplitConfig carries
// no YAML tags, so a default_split_config written as YAML arrives with its type
// set and its line count zero.
func SplitProducesChapters(config shelf.SplitConfig) bool {
	switch config.Type {
	case shelf.SplitTypeLineCount:
		return config.LineCount > 0
	case shelf.SplitTypeRegex:
		return trimJSSpace(config.Regex) != ""
	case shelf.SplitTypeBoundary:
		return len(config.Boundaries) > 0
	default:
		return false
	}
}

// PlanUpgrade decides what a legacy source needs from its bytes alone.
//
// effectiveSplit is the source's own split configuration, or the shelf-wide
// default when the source has none. effectiveFormat is the format the source
// renders as today.
func PlanUpgrade(content string, effectiveSplit shelf.SplitConfig, effectiveFormat string) (Plan, error) {
	if effectiveSplit.Type != shelf.SplitTypeNone {
		switch effectiveSplit.Type {
		case shelf.SplitTypeLineCount, shelf.SplitTypeRegex, shelf.SplitTypeBoundary:
		default:
			return Plan{}, util.Errorf("%w: %q", ErrUnrecognizedSplitType, effectiveSplit.Type)
		}
	}

	headings := ScanMarkdownH2Headings(content)

	if !SplitProducesChapters(effectiveSplit) {
		// Nothing is splitting this source today, so its bytes already say
		// everything its chapters do. Claim the format and leave the text alone.
		chapters := 1
		if effectiveFormat == shelf.BookFormatMarkdown && len(headings) > 0 {
			chapters = len(headings)
		}
		plan := Plan{Format: effectiveFormat, Chapters: chapters}
		if effectiveSplit.Type != shelf.SplitTypeNone {
			plan.Reason = "split configuration names no chapter boundary"
		}
		return plan, nil
	}

	if len(headings) > 0 {
		// The text already expresses the chapters the split configuration was
		// asking for. Only the format changes, and only on paper — though a
		// plain-text source starts rendering those lines as headings.
		return Plan{
			Format:   shelf.BookFormatMarkdown,
			Chapters: len(headings),
			Reason:   "content already contains H2 headings",
		}, nil
	}

	upgraded, err := UpgradeLegacyToMarkdown(content, effectiveSplit)
	if err != nil {
		return Plan{}, util.Errorf("%w", err)
	}

	return Plan{
		Format:   shelf.BookFormatMarkdown,
		Content:  upgraded,
		Rewrite:  true,
		Chapters: len(ScanMarkdownH2Headings(upgraded)),
	}, nil
}
