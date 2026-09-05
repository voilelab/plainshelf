/**
 * Turns untrusted book text into something safe to put on screen.
 *
 * A book's description mixes hand-written Markdown with whatever HTML an EPUB
 * import carried over, so no view may interpolate it as it stands. Two outputs:
 * `toPlainSummary` for the places that want words, and `renderDescriptionHtml`
 * followed by `sanitizeHtml` for the places that render the markup, with
 * `SafeHtml.vue` as the only permitted `v-html` sink. `renderDescription`
 * answers both from one parse.
 */
import createSanitizer, { type DOMPurify } from 'dompurify';
import MarkdownIt from 'markdown-it';

/**
 * The dialect every reading of a book's text is parsed in: raw HTML kept, a
 * single newline is a line break. Stated once because the reader's chapters
 * (`renderMarkdownBlocks`) are the same kind of text; the instances cannot be
 * shared, because each configures rules of its own on top.
 */
export const BOOK_TEXT_MARKDOWN_OPTIONS = {
  html: true,
  breaks: true,
  linkify: false,
  typographer: false
} as const;

/**
 * Links stay enabled here, unlike in the reader: only the link text survives
 * sanitizing anyway, and leaving the rule off would spell it out as
 * `[text](target)`. One instance serves both outputs, so a card's words and a
 * detail page's markup are the same parse of the same source.
 */
const markdown = new MarkdownIt(BOOK_TEXT_MARKDOWN_OPTIONS);

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
 * sanitizer drops these subtrees whole, so a summary that read them would report
 * words no rendered description can ever show. One list serves both readings for
 * that reason.
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
 * A parser rather than a tag-shaped regular expression, so a `<` that is a real
 * less-than sign keeps the text that follows it.
 */
function textOf(html: string): string {
  // The parsed document has no browsing context, which is what makes reading it
  // back safe. That property belongs to DOMParser, not to the traversal below:
  // the same markup assigned to a detached element's innerHTML does fetch an
  // `<img>` and a `<video>` poster, letting a description PlainShelf did not
  // write call home the moment a card is drawn.
  const document = new DOMParser().parseFromString(html, 'text/html');
  const parts: string[] = [];
  collectText(document.body, parts);
  return parts.join('').replace(/\s+/g, ' ').trim();
}

/**
 * One line, no HTML tags, no Markdown syntax left over. Text that amounts to no
 * words - `<br>`, an empty tag, spaces - returns an empty string, which is the
 * caller's cue to fall back to something else.
 */
export function toPlainSummary(source: string | null | undefined): string {
  const html = renderDescriptionHtml(source);
  if (!html) {
    return '';
  }

  return textOf(html);
}

/**
 * Renderer output and nothing more: the dialect above keeps raw HTML by design,
 * so whatever an EPUB import carried over is still in it and no view may put it
 * on screen unsanitized.
 *
 * Links and images are left to the profile rather than disabled here: it allows
 * neither tag and keeps their contents, so a link arrives as its text - the same
 * words `toPlainSummary` reports - and an image as nothing.
 */
export function renderDescriptionHtml(source: string | null | undefined): string {
  if (!source || !source.trim()) {
    return '';
  }

  return markdown.render(source);
}

interface RenderedDescription {
  /** Renderer output, for `SafeHtml` to sanitize; never a `v-html` sink itself. */
  html: string;
  /** That same markup read back as words, empty when it amounts to none. */
  text: string;
}

/**
 * Both readings from one parse, for a view that needs to decide whether there is
 * anything to show *and* show it. Deciding on `text` and rendering `html` keeps
 * the two questions answering from the same source.
 */
export function renderDescription(source: string | null | undefined): RenderedDescription {
  const html = renderDescriptionHtml(source);
  return { html, text: html ? textOf(html) : '' };
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
    allowedAttr: [...LIST_ATTR, 'title', 'style'],
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
 * Hooks are per-instance global state in DOMPurify, so one shared instance could
 * serve only one class allowlist and the profiles would leak into each other.
 * Creation is lazy because the hook and `sanitizeColorStyle` both need a DOM.
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
