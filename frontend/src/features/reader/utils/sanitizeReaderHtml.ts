import DOMPurify from 'dompurify';

const ALLOWED_TAGS = [
  'details',
  'summary',
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
  'dd',
  'table',
  'caption',
  'thead',
  'tbody',
  'tfoot',
  'tr',
  'th',
  'td'
];

const ALLOWED_ATTR = [
  'open',
  'start',
  'reversed',
  'type',
  'colspan',
  'rowspan',
  'scope',
  'headers',
  'title',
  'style',
  'class'
];

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

const UNSAFE_COLOR_VALUE_RE = /(?:url|var|expression)\s*\(|[{}\\@!]|\/\*/i;

/** Returns a canonical color-only style or an empty string when none is safe. */
export function sanitizeReaderColorStyle(rawStyle: string): string {
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

DOMPurify.addHook('uponSanitizeAttribute', (_node, data) => {
  if (data.attrName === 'style') {
    const style = sanitizeReaderColorStyle(data.attrValue);
    data.keepAttr = Boolean(style);
    data.attrValue = style;
    return;
  }

  if (data.attrName === 'class') {
    const classes = data.attrValue.split(/\s+/).filter((name) => READER_CLASSES.has(name));
    data.keepAttr = classes.length > 0;
    data.attrValue = classes.join(' ');
  }
});

export function sanitizeReaderHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
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
