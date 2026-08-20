/**
 * Turns untrusted book text into something safe to put on screen.
 *
 * A book's description is a mix of hand-written Markdown and whatever HTML an
 * EPUB import carried over, and the reader renders whole chapters of the same
 * kind of text, so no view may interpolate either as it stands. Two kinds of
 * output come out of here: `toPlainSummary` for the places that want words —
 * the card and the list summaries print `<p>` at the reader today, and a `<ul>`
 * or an `<h1>` resizes a card the grid needs to keep at one height — and, for
 * the places that render the markup, `renderDescriptionHtml` followed by
 * `sanitizeHtml`, with `SafeHtml.vue` as the only permitted `v-html` sink.
 *
 * Every caller that needs the description as words goes through this module,
 * which is what keeps a summary and the description itself reading the same
 * source the same way.
 */
import createSanitizer, { type DOMPurify } from 'dompurify';
import MarkdownIt from 'markdown-it';

/**
 * The dialect a description is written in: raw HTML kept, a single newline is
 * a line break. Links stay enabled, unlike in the reader, which refuses to
 * render a navigable anchor - here only the link text survives anyway, and
 * leaving the rule off would spell it out as `[text](target)`.
 *
 * One instance serves both outputs, so the words a card reports and the markup
 * a detail page renders are the same parse of the same source.
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

/**
 * Elements holding markup or a document of their own rather than prose. The
 * sanitizer drops these subtrees whole, contents included, so a summary that
 * read them would report words no rendered description can ever show - and a
 * detail page would label a row whose sanitized body is empty. One list serves
 * both readings for that reason; they cannot be allowed to disagree.
 */
const NON_PROSE_ELEMENTS = [
  'embed',
  'iframe',
  'math',
  'noscript',
  'object',
  'script',
  'style',
  'svg',
  'template'
];

const NON_PROSE_TAG_NAMES = new Set(NON_PROSE_ELEMENTS.map((name) => name.toUpperCase()));

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
    // `tagName` is uppercased for HTML elements only: an `<svg>` or a `<math>`
    // belongs to another namespace and reports the case it was written in.
    if (NON_PROSE_TAG_NAMES.has(element.tagName.toUpperCase())) {
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

/**
 * Renders a description as the markup it describes, for `SafeHtml` to sanitize
 * with the `summary` profile. The return value is renderer output and nothing
 * more: the dialect above keeps raw HTML by design, so whatever an EPUB import
 * carried over is still in it, and no view may put it on screen unsanitized.
 *
 * A link and an image are left to the profile rather than disabled here. The
 * profile allows neither tag and keeps their contents, so a link arrives as its
 * text - the same words `toPlainSummary` would report - while an image, which
 * has no text, arrives as nothing.
 */
export function renderDescriptionHtml(source: string | null | undefined): string {
  if (!source || !source.trim()) {
    return '';
  }

  return markdown.render(source);
}

export type SafeHtmlProfile = 'reader' | 'summary';

interface SafeHtmlProfileConfig {
  allowedTags: string[];
  allowedAttr: string[];
  /** Class names kept on any element; every other name is dropped. */
  allowedClasses: ReadonlySet<string>;
}

const INLINE_TAGS = [
  'p',
  'div',
  'section',
  'article',
  'blockquote',
  'pre',
  'span',
  'strong',
  'b',
  'em',
  'i',
  'u',
  's',
  'del',
  'mark',
  'small',
  'sub',
  'sup',
  'code',
  'kbd',
  'br',
  'hr',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'ul',
  'ol',
  'li',
  'dl',
  'dt',
  'dd'
];

const TABLE_TAGS = ['table', 'caption', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td'];

const DISCLOSURE_TAGS = ['details', 'summary'];

const TABLE_ATTR = ['colspan', 'rowspan', 'scope', 'headers'];

const LIST_ATTR = ['start', 'reversed', 'type'];

const READER_CLASSES = new Set([
  'reader-text-block',
  'reader-text-quote',
  'reader-md-h1',
  'reader-md-h2',
  'reader-md-h3',
  'reader-md-list',
  'reader-md-code',
  'reader-md-inline-code',
  'reader-md-hr',
  'reader-asset-slot'
]);

const PROFILES: Record<SafeHtmlProfile, SafeHtmlProfileConfig> = {
  reader: {
    allowedTags: [...DISCLOSURE_TAGS, ...INLINE_TAGS, ...TABLE_TAGS],
    allowedAttr: ['open', ...LIST_ATTR, ...TABLE_ATTR, 'title', 'style', 'class'],
    allowedClasses: READER_CLASSES
  },
  // A summary is prose shown beside other UI: no disclosure widgets, no
  // tables, and no class names, which also drops the reader's asset slots.
  summary: {
    allowedTags: INLINE_TAGS,
    allowedAttr: [...LIST_ATTR, 'title', 'style', 'class'],
    allowedClasses: new Set<string>()
  }
};

const UNSAFE_COLOR_VALUE_RE = /(?:url|var|expression)\s*\(|[{}\\@!]|\/\*/i;

/** Returns a canonical color-only style or an empty string when none is safe. */
export function sanitizeColorStyle(rawStyle: string): string {
  let safeColor = '';

  for (const declaration of rawStyle.split(';')) {
    const colon = declaration.indexOf(':');
    if (colon < 0 || declaration.slice(0, colon).trim().toLowerCase() !== 'color') continue;

    const value = declaration.slice(colon + 1).trim();
    if (!value || UNSAFE_COLOR_VALUE_RE.test(value)) continue;

    const probe = document.createElement('span');
    probe.style.color = value;
    if (probe.style.color) safeColor = probe.style.color;
  }

  return safeColor ? `color: ${safeColor}` : '';
}

/**
 * Hooks are per-instance global state in DOMPurify, so a single shared instance
 * could only serve one class allowlist at a time and the profiles would leak
 * into each other. Each profile therefore owns its instance and its hook.
 * Creation is lazy because the hook and `sanitizeColorStyle` both need a DOM,
 * which a module-level side effect would demand at import time.
 */
const instances = new Map<SafeHtmlProfile, DOMPurify>();

function instanceFor(profile: SafeHtmlProfile): DOMPurify {
  const existing = instances.get(profile);
  if (existing) return existing;

  const { allowedClasses } = PROFILES[profile];
  const purify = createSanitizer(window);

  purify.addHook('uponSanitizeAttribute', (_node, data) => {
    if (data.attrName === 'style') {
      const style = sanitizeColorStyle(data.attrValue);
      data.keepAttr = Boolean(style);
      data.attrValue = style;
      return;
    }

    if (data.attrName === 'class') {
      const classes = data.attrValue.split(/\s+/).filter((name) => allowedClasses.has(name));
      data.keepAttr = classes.length > 0;
      data.attrValue = classes.join(' ');
    }
  });

  instances.set(profile, purify);
  return purify;
}

export function sanitizeHtml(html: string, profile: SafeHtmlProfile): string {
  const { allowedTags, allowedAttr } = PROFILES[profile];
  return instanceFor(profile).sanitize(html, {
    ALLOWED_TAGS: allowedTags,
    ALLOWED_ATTR: allowedAttr,
    ALLOW_ARIA_ATTR: false,
    ALLOW_DATA_ATTR: false,
    ALLOW_UNKNOWN_PROTOCOLS: false,
    KEEP_CONTENT: true,
    FORBID_CONTENTS: NON_PROSE_ELEMENTS
  });
}
