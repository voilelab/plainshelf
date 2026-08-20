// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { renderDescriptionHtml, sanitizeColorStyle, sanitizeHtml, toPlainSummary } from './safeHtml';

/** Every case in the sanitizer blocks is the reader profile unless it says otherwise. */
const sanitizeReaderHtml = (html: string): string => sanitizeHtml(html, 'reader');

describe('toPlainSummary', () => {
  it('drops the HTML tags an EPUB description arrives with', () => {
    expect(toPlainSummary('<p>這本書在講一個人。</p>')).toBe('這本書在講一個人。');
  });

  it('resolves Markdown syntax instead of showing it', () => {
    const source = '# 標題\n\n**粗體**與*斜體*\n\n- 項目一\n- 項目二';
    expect(toPlainSummary(source)).toBe('標題 粗體與斜體 項目一 項目二');
  });

  it('keeps a link as its text', () => {
    expect(toPlainSummary('見[官方網站](https://example.com)。')).toBe('見官方網站。');
  });

  it('separates blocks instead of running them together', () => {
    expect(toPlainSummary('<p>第一段</p><p>第二段</p>')).toBe('第一段 第二段');
    expect(toPlainSummary('<ul><li>一</li><li>二</li></ul>')).toBe('一 二');
  });

  it('collapses every line break into a single line', () => {
    const summary = toPlainSummary('<h1>書名</h1>\r\n<p>第一行<br>第二行</p>\n<blockquote>引用</blockquote>');
    expect(summary).toBe('書名 第一行 第二行 引用');
    expect(summary).not.toMatch(/\s{2}|[\r\n]/);
  });

  it('keeps text that follows a real less-than sign', () => {
    expect(toPlainSummary('5 < 10，而 a<b 也成立')).toBe('5 < 10，而 a<b 也成立');
  });

  it('leaves out markup that is not prose', () => {
    expect(toPlainSummary('<style>p { color: red }</style><p>簡介</p><script>alert(1)</script>')).toBe('簡介');
  });

  // A summary that read a subtree the sanitizer deletes would report words the
  // detail page never shows, and would make an empty block look worth a row.
  it('reads nothing out of a subtree the sanitizer deletes whole', () => {
    for (const source of ['<svg><text>Hello</text></svg>', '<math><mi>x</mi></math>']) {
      expect(toPlainSummary(source), source).toBe('');
      expect(sanitizeHtml(renderDescriptionHtml(source), 'summary').replace(/<[^>]+>|\s/g, ''), source)
        .toBe('');
    }
  });

  it('reports a description that amounts to no words as empty', () => {
    expect(toPlainSummary('<br>')).toBe('');
    expect(toPlainSummary('<p> </p><div></div>')).toBe('');
    expect(toPlainSummary('   ')).toBe('');
    expect(toPlainSummary('')).toBe('');
    expect(toPlainSummary(undefined)).toBe('');
  });

  it('leaves plain prose alone', () => {
    expect(toPlainSummary('一本沒有任何標記的簡介')).toBe('一本沒有任何標記的簡介');
  });
});

describe('sanitizeColorStyle', () => {
  it('keeps only the last valid color declaration', () => {
    expect(sanitizeColorStyle('position: fixed; color: purple; color: rgb(0, 0, 255)'))
      .toBe('color: rgb(0, 0, 255)');
  });

  it('rejects resource, variable, expression, and important values', () => {
    for (const value of [
      'color: url(https://example.com/x)',
      'color: var(--secret)',
      'color: expression(alert(1))',
      'color: red !important',
      'background: red'
    ]) {
      expect(sanitizeColorStyle(value), value).toBe('');
    }
  });

  // The declaration is rejected on the name, not on the CSS parser's verdict:
  // `var(...)` is a value the parser accepts and would hand back verbatim, and
  // a parser that also tolerates a space before `(` would let the same
  // reference through. The last case is one such value here.
  it('rejects a functional value whose name is separated from its parenthesis', () => {
    for (const value of [
      'color: url (https://example.com/x)',
      'color: var (--secret)',
      'color: expression (alert(1))',
      'color: color(display-p3 var (--secret) 0 0)'
    ]) {
      expect(sanitizeColorStyle(value), value).toBe('');
    }
  });

  // The regex guards a style attribute, so over-blocking would silently strip
  // an author's colors; these spellings all have to survive it.
  it('keeps every spelling of a plain color the CSS parser understands', () => {
    expect(sanitizeColorStyle('color: RED')).toBe('color: red');
    expect(sanitizeColorStyle('color: #abc')).toBe('color: rgb(170, 187, 204)');
    expect(sanitizeColorStyle('color: rgb(1, 2, 3)')).toBe('color: rgb(1, 2, 3)');
    expect(sanitizeColorStyle('color: rgba(1, 2, 3, 0.5)')).toBe('color: rgba(1, 2, 3, 0.5)');
    expect(sanitizeColorStyle('COLOR: purple')).toBe('color: purple');
  });

  // A later unparseable value must not take the earlier one down with it,
  // otherwise one typo further along the attribute silently drops the color.
  it('keeps the last color the parser accepted, not the last one written', () => {
    expect(sanitizeColorStyle('color: purple; color: bogus')).toBe('color: purple');
    expect(sanitizeColorStyle('color: purple; color: var(--secret)')).toBe('color: purple');
  });
});

describe('sanitizeHtml, reader profile', () => {
  it('keeps reading markup, details state, tables, and safe colors', () => {
    const clean = sanitizeReaderHtml([
      '<details open><summary>Info</summary><p style="color: purple; position: fixed">Body</p></details>',
      '<table><caption>Stats</caption><tbody><tr><th scope="row">MP</th><td colspan="2">95%</td></tr></tbody></table>'
    ].join(''));

    expect(clean).toContain('<details open=""><summary>Info</summary>');
    expect(clean).toContain('<p style="color: purple">Body</p>');
    expect(clean).toContain('<table>');
    expect(clean).toContain('scope="row"');
    expect(clean).toContain('colspan="2"');
  });

  it('unwraps plot while preserving its readable content', () => {
    const clean = sanitizeReaderHtml('<plot secret="x">Story <strong>continues</strong></plot>');
    expect(clean).toBe('Story <strong>continues</strong>');
    expect(clean).not.toContain('plot');
  });

  it('removes scripts, active content, URL-bearing media, and event handlers', () => {
    const clean = sanitizeReaderHtml([
      '<script>alert(1)</script><style>body{display:none}</style>',
      '<p onclick="alert(1)" id="app" data-x="1" aria-label="x">Safe</p>',
      '<a href="javascript:alert(1)">link text</a>',
      '<img src="https://example.com/x.png" onerror="alert(1)">',
      '<iframe src="https://example.com"></iframe>',
      '<svg onload="alert(1)"><script>alert(1)</script></svg>',
      '<form action="https://example.com"><input name="secret"><button>Send</button></form>'
    ].join(''));

    expect(clean).toContain('<p>Safe</p>');
    expect(clean).toContain('link text');
    expect(clean).toContain('Send');
    for (const forbidden of [
      '<script', '<style', '<a', '<img', '<iframe', '<svg', '<form', '<input', '<button',
      'onclick=', 'onerror=', 'href=', 'src=', 'id=', 'data-x=', 'aria-label='
    ]) {
      expect(clean, forbidden).not.toContain(forbidden);
    }
  });

  it('keeps only renderer-owned classes', () => {
    const clean = sanitizeReaderHtml(
      '<p class="evil reader-text-block reader-md-code">Text</p>'
    );
    expect(clean).toBe('<p class="reader-text-block reader-md-code">Text</p>');
  });

  // Dropping the names but keeping an empty attribute would leave `class=""`
  // on every styled element the source authored, and any empty name reaching
  // the allowlist would make that the normal outcome.
  it('drops a class attribute that names nothing the renderer owns', () => {
    expect(sanitizeReaderHtml('<p class="evil">Text</p>')).toBe('<p>Text</p>');
    expect(sanitizeReaderHtml('<p class=" evil ">Text</p>')).toBe('<p>Text</p>');
    expect(sanitizeReaderHtml('<p class="">Text</p>')).toBe('<p>Text</p>');
  });

  // These class names are the contract between the Markdown renderer and the
  // reader stylesheet: one lost from the allowlist is unstyled output, with no
  // error anywhere.
  it('keeps every class name the renderer is allowed to emit', () => {
    for (const name of [
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
    ]) {
      expect(sanitizeReaderHtml(`<span class="${name}">Text</span>`), name)
        .toBe(`<span class="${name}">Text</span>`);
    }
  });

  // The tag list is the reader's markup vocabulary. Losing one entry unwraps
  // that element everywhere without failing anything else.
  it('keeps every tag the reader promises to render', () => {
    for (const tag of [
      'div', 'section', 'article', 'blockquote', 'pre', 'span',
      'strong', 'b', 'em', 'i', 'u', 's', 'del', 'mark', 'small', 'sub', 'sup',
      'code', 'kbd', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'dl', 'dt', 'dd'
    ]) {
      expect(sanitizeReaderHtml(`<${tag}>Text</${tag}>`), tag)
        .toBe(`<${tag}>Text</${tag}>`);
    }
    expect(sanitizeReaderHtml('<hr><br>')).toBe('<hr><br>');
    expect(sanitizeReaderHtml(
      '<table><caption>C</caption><thead><tr><th>H</th></tr></thead>'
      + '<tbody><tr><td>D</td></tr></tbody><tfoot><tr><td>F</td></tr></tfoot></table>'
    )).toBe(
      '<table><caption>C</caption><thead><tr><th>H</th></tr></thead>'
      + '<tbody><tr><td>D</td></tr></tbody><tfoot><tr><td>F</td></tr></tfoot></table>'
    );
  });

  // Structure a book actually uses: an author's numbering and a table's own
  // cell wiring are lost if these attributes stop being allowed.
  it('keeps the list and table attributes that carry meaning', () => {
    expect(sanitizeReaderHtml('<ol start="3" reversed type="a"><li>x</li></ol>'))
      .toBe('<ol start="3" reversed="" type="a"><li>x</li></ol>');
    expect(sanitizeReaderHtml('<table><tr><th headers="a">H</th><td rowspan="2">D</td></tr></table>'))
      .toBe('<table><tbody><tr><th headers="a">H</th><td rowspan="2">D</td></tr></tbody></table>');
  });

  // `span` is a <col>/<colgroup> attribute and neither tag is allowed here, so
  // on any tag the reader can render it is noise the sanitizer should remove.
  it('drops the span attribute while keeping the table attributes beside it', () => {
    expect(sanitizeReaderHtml('<table><tr><td span="2" colspan="2">D</td></tr></table>'))
      .toBe('<table><tbody><tr><td colspan="2">D</td></tr></tbody></table>');
    expect(sanitizeReaderHtml('<table><tr><th span="2" scope="col" headers="a">H</th></tr></table>'))
      .toBe('<table><tbody><tr><th scope="col" headers="a">H</th></tr></tbody></table>');
    expect(sanitizeReaderHtml('<span span="2">Text</span>')).toBe('<span>Text</span>');
  });

  // Unwrapping an unknown element keeps its prose on purpose, but the text
  // inside active content is markup for another engine, never reading matter.
  it('drops the text inside active content instead of unwrapping it', () => {
    for (const tag of ['script', 'style', 'iframe', 'object', 'svg', 'math']) {
      expect(sanitizeReaderHtml(`<${tag}>LEAK</${tag}>`), tag).toBe('');
    }
  });

  it('keeps the renderer-owned asset slot without allowing arbitrary classes', () => {
    const clean = sanitizeReaderHtml(
      '<span class="evil reader-asset-slot" title="slot-token" data-name="map.png"></span>'
    );
    expect(clean).toBe('<span class="reader-asset-slot" title="slot-token"></span>');
  });

  it('safely repairs malformed markup', () => {
    const clean = sanitizeReaderHtml('<details><summary>Info<script>alert(1)</script><p>Body');
    expect(clean).toContain('<details>');
    expect(clean).toContain('<summary>Info');
    expect(clean).toContain('<p>Body</p>');
    expect(clean).toContain('</summary></details>');
    expect(clean).not.toContain('script');
    expect(clean).not.toContain('alert(1)');
  });
});

describe('sanitizeHtml, summary profile', () => {
  it('drops disclosure widgets and tables but keeps their text', () => {
    expect(sanitizeHtml('<details><summary>Title</summary><p>Body</p></details>', 'summary'))
      .toBe('Title<p>Body</p>');
    expect(sanitizeHtml('<table><caption>C</caption><tr><td>D</td></tr></table>', 'summary'))
      .toBe('CD');
  });

  it('keeps the prose elements a summary is written in', () => {
    expect(sanitizeHtml('<p>Text <strong>bold</strong> <em>italic</em></p>', 'summary'))
      .toBe('<p>Text <strong>bold</strong> <em>italic</em></p>');
    expect(sanitizeHtml('<ol start="3"><li>x</li></ol>', 'summary'))
      .toBe('<ol start="3"><li>x</li></ol>');
  });

  it('keeps color-only styles and still refuses everything else', () => {
    expect(sanitizeHtml('<span style="position: fixed; color: red">T</span>', 'summary'))
      .toBe('<span style="color: red">T</span>');
    expect(sanitizeHtml('<p onclick="alert(1)">T</p>', 'summary')).toBe('<p>T</p>');
    expect(sanitizeHtml('<script>alert(1)</script>', 'summary')).toBe('');
  });

  // The class allowlist used to be a module-level DOMPurify hook shared by every
  // caller. Interleaving the profiles is what a single hook would get wrong.
  it('does not leak its class allowlist into the reader profile, in either order', () => {
    const markup = '<h2 class="reader-md-h2">Heading</h2>';
    const results = [
      sanitizeHtml(markup, 'summary'),
      sanitizeHtml(markup, 'reader'),
      sanitizeHtml(markup, 'summary'),
      sanitizeHtml(markup, 'reader')
    ];
    expect(results).toEqual([
      '<h2>Heading</h2>',
      '<h2 class="reader-md-h2">Heading</h2>',
      '<h2>Heading</h2>',
      '<h2 class="reader-md-h2">Heading</h2>'
    ]);
  });

  it('drops the reader asset slot rather than leaving an empty element', () => {
    expect(sanitizeHtml('<span class="reader-asset-slot" title="tok"></span>', 'summary'))
      .toBe('<span title="tok"></span>');
  });
});

/**
 * The two steps the detail page runs together: the description is rendered as
 * the document it is, then sanitized under the profile that decides what a
 * summary may contain. Neither step is safe on its own, so the cases below
 * assert on the pair.
 */
const renderDescription = (source: string): string =>
  sanitizeHtml(renderDescriptionHtml(source), 'summary');

describe('renderDescriptionHtml', () => {
  it('renders the Markdown a description is hand-written in', () => {
    expect(renderDescription('**粗體**與*斜體*')).toBe('<p><strong>粗體</strong>與<em>斜體</em></p>\n');
    expect(renderDescription('第一行\n第二行')).toBe('<p>第一行<br>\n第二行</p>\n');
    expect(renderDescription('- 項目一\n- 項目二'))
      .toBe('<ul>\n<li>項目一</li>\n<li>項目二</li>\n</ul>\n');
    expect(renderDescription('> 引文')).toBe('<blockquote>\n<p>引文</p>\n</blockquote>\n');
  });

  // The paragraphs an EPUB import carried over are already HTML on disk, which
  // is what the detail page used to print as text.
  it('renders the HTML an EPUB description arrives as', () => {
    expect(renderDescription('<p>第一段</p><p>第二段</p>')).toBe('<p>第一段</p><p>第二段</p>');
  });

  it('strips active content without taking the words beside it', () => {
    expect(renderDescription('簡介<script>alert(1)</script>結束')).toBe('<p>簡介結束</p>\n');
    expect(renderDescription('<img src=x onerror=alert(1)>看得到的字')).toBe('<p>看得到的字</p>\n');
    expect(renderDescription('<a href="javascript:alert(1)">連結文字</a>後面'))
      .toBe('<p>連結文字後面</p>\n');
    expect(renderDescription('<iframe src="https://example.com"></iframe>後面')).toBe('後面');
    expect(renderDescription('<style>p { color: red }</style>正文')).toBe('正文');
  });

  // A target the dialect refuses is never a link to begin with, so nothing
  // downstream has to recognize the scheme.
  it('leaves a Markdown link to a script target as the text it was written as', () => {
    expect(renderDescription('[連結](javascript:alert(1))')).toBe('<p>[連結](javascript:alert(1))</p>\n');
  });

  it('renders nothing for a description with nothing in it', () => {
    expect(renderDescriptionHtml('')).toBe('');
    expect(renderDescriptionHtml('   ')).toBe('');
    expect(renderDescriptionHtml(null)).toBe('');
    expect(renderDescriptionHtml(undefined)).toBe('');
  });
});
