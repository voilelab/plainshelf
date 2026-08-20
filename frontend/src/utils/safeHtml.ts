/**
 * Turns untrusted text into a DOM-safe HTML string.
 *
 * Rendering happens in `renderMarkdownBlocks.ts`; this module is the sanitizer
 * every caller must pass that output through, and `SafeHtml.vue` is the only
 * permitted `v-html` sink. Callers differ in how much markup they can afford to
 * show, so the allowlist is a *profile* rather than a second sanitizer: the
 * reader renders a whole chapter, while a book summary is a paragraph of prose
 * next to other UI and must not grow disclosure widgets, tables, or the
 * reader's own class names.
 */
import createSanitizer, { type DOMPurify } from 'dompurify';
import { renderMarkdownHtml } from './renderMarkdownBlocks';

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
    FORBID_CONTENTS: [
      'script',
      'style',
      'template',
      'noscript',
      'iframe',
      'object',
      'embed',
      'svg',
      'math'
    ]
  });
}

/**
 * Collapses a Markdown passage into one line of plain text.
 *
 * It goes through the same render and sanitize steps as the rendered summary,
 * so a card and the detail page below it cannot disagree about what the text
 * says. Parsing into an inert document keeps the markup away from a live DOM.
 */
export function toPlainSummary(source: string): string {
  const html = sanitizeHtml(renderMarkdownHtml(source), 'summary');
  if (!html) return '';
  const parsed = new DOMParser().parseFromString(html, 'text/html');
  return (parsed.body.textContent ?? '').replace(/\s+/g, ' ').trim();
}
