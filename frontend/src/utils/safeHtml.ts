import createSanitizer, { type DOMPurify } from 'dompurify';
import MarkdownIt from 'markdown-it';

export const BOOK_TEXT_MARKDOWN_OPTIONS = {
  html: true,
  breaks: true,
  linkify: false,
  typographer: false
} as const;

const markdown = new MarkdownIt(BOOK_TEXT_MARKDOWN_OPTIONS);

const SEPARATING_ELEMENTS = new Set([
  'ADDRESS', 'ARTICLE', 'ASIDE', 'BLOCKQUOTE', 'BR', 'DD', 'DIV', 'DL', 'DT', 'FIELDSET',
  'FIGCAPTION', 'FIGURE', 'FOOTER', 'FORM', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HEADER',
  'HR', 'LI', 'MAIN', 'NAV', 'OL', 'P', 'PRE', 'SECTION', 'TABLE', 'TBODY', 'TD', 'TFOOT',
  'TH', 'THEAD', 'TR', 'UL'
]);

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

function textOf(html: string): string {
  // HTML documents created by DOMParser are inactive; detached innerHTML in the live document can still fetch image and poster URLs (Chromium reproduction, 8941f45e).
  const document = new DOMParser().parseFromString(html, 'text/html');
  const parts: string[] = [];
  collectText(document.body, parts);
  return parts.join('').replace(/\s+/g, ' ').trim();
}

export function toPlainSummary(source: string | null | undefined): string {
  const html = renderDescriptionHtml(source);
  if (!html) {
    return '';
  }

  return textOf(html);
}

export function renderDescriptionHtml(source: string | null | undefined): string {
  if (!source || !source.trim()) {
    return '';
  }

  return markdown.render(source);
}

interface RenderedDescription {
  html: string;
  text: string;
}

export function renderDescription(source: string | null | undefined): RenderedDescription {
  const html = renderDescriptionHtml(source);
  return { html, text: html ? textOf(html) : '' };
}

export type SafeHtmlProfile = 'reader' | 'summary';

interface SafeHtmlProfileConfig {
  allowedTags: string[];
  allowedAttr: string[];
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
  summary: {
    allowedTags: INLINE_TAGS,
    allowedAttr: [...LIST_ATTR, 'title', 'style'],
    allowedClasses: new Set<string>()
  }
};

const UNSAFE_COLOR_VALUE_RE = /(?:url|var|expression)\s*\(|[{}\\@!]|\/\*/i;

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

// DOMPurify 3.4.16 stores hooks per sanitizer instance; addHook appends to that instance (purify.es.mjs).
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
