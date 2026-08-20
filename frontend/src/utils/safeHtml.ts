/**
 * Turns untrusted book text into something safe to put on screen.
 *
 * A book's description is a mix of hand-written Markdown and whatever HTML an
 * EPUB import carried over, so no view may interpolate it as it stands: the
 * card and list summaries print `<p>` at the reader today. Rendering it there
 * is not the answer either - a `<ul>` or an `<h1>` resizes a card that the
 * grid needs to keep at one height.
 *
 * Every caller that needs the description as words goes through this module,
 * which is what keeps a summary and the description itself reading the same
 * source the same way. Plain text is the only output it produces so far.
 */
import MarkdownIt from 'markdown-it';

/**
 * The dialect a description is written in: raw HTML kept, a single newline is
 * a line break. Links stay enabled, unlike in the reader, which refuses to
 * render a navigable anchor - here only the link text survives anyway, and
 * leaving the rule off would spell it out as `[text](target)`.
 */
const markdown = new MarkdownIt({
  html: true,
  breaks: true,
  linkify: false,
  typographer: false
});

/**
 * Elements whose boundaries separate words. Two paragraphs must not read as
 * `第一段第二段`, while an `<em>` inside a sentence must not gain a space it
 * never had - CJK prose has no word spacing to hide one in.
 */
const SEPARATING_ELEMENTS = new Set([
  'ADDRESS', 'ARTICLE', 'ASIDE', 'BLOCKQUOTE', 'BR', 'DD', 'DIV', 'DL', 'DT', 'FIELDSET',
  'FIGCAPTION', 'FIGURE', 'FOOTER', 'FORM', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HEADER',
  'HR', 'LI', 'MAIN', 'NAV', 'OL', 'P', 'PRE', 'SECTION', 'TABLE', 'TBODY', 'TD', 'TFOOT',
  'TH', 'THEAD', 'TR', 'UL'
]);

/** Elements whose text is markup rather than prose, and never part of a summary. */
const NON_PROSE_ELEMENTS = new Set(['IFRAME', 'NOSCRIPT', 'OBJECT', 'SCRIPT', 'STYLE', 'TEMPLATE']);

function collectText(node: Node, parts: string[]): void {
  for (const child of Array.from(node.childNodes)) {
    if (child.nodeType === child.TEXT_NODE) {
      parts.push(child.nodeValue ?? '');
      continue;
    }
    if (child.nodeType !== child.ELEMENT_NODE) {
      continue;
    }

    const element = child as Element;
    if (NON_PROSE_ELEMENTS.has(element.tagName)) {
      continue;
    }

    const separates = SEPARATING_ELEMENTS.has(element.tagName);
    if (separates) {
      parts.push(' ');
    }
    collectText(element, parts);
    if (separates) {
      parts.push(' ');
    }
  }
}

/**
 * Reads `source` as the document it describes and returns its words: one line,
 * no HTML tags, no Markdown syntax left over. A parser rather than a tag-shaped
 * regular expression, so a `<` that is a real less-than sign keeps the text
 * that follows it.
 *
 * Text that amounts to no words - `<br>`, an empty tag, spaces - returns an
 * empty string, which is the caller's cue to fall back to something else.
 */
export function toPlainSummary(source: string | null | undefined): string {
  if (!source || !source.trim()) {
    return '';
  }

  // The parsed document has no browsing context: scripts do not run and no
  // element loads anything, which is what makes reading it back safe. That
  // property belongs to DOMParser rather than to the traversal below - the
  // same markup assigned to a detached element's innerHTML does fetch an
  // `<img>`, a `<video>` poster and its `<source>`, which would let a
  // description PlainShelf did not write call home the moment a card is drawn.
  const document = new DOMParser().parseFromString(markdown.render(source), 'text/html');
  const parts: string[] = [];
  collectText(document.body, parts);
  return parts.join('').replace(/\s+/g, ' ').trim();
}
