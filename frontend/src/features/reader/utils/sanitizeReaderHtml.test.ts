// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { sanitizeReaderColorStyle, sanitizeReaderHtml } from './sanitizeReaderHtml';

describe('sanitizeReaderColorStyle', () => {
  it('keeps only the last valid color declaration', () => {
    expect(sanitizeReaderColorStyle('position: fixed; color: purple; color: rgb(0, 0, 255)'))
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
      expect(sanitizeReaderColorStyle(value), value).toBe('');
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
      expect(sanitizeReaderColorStyle(value), value).toBe('');
    }
  });

  // The regex guards a style attribute, so over-blocking would silently strip
  // an author's colors; these spellings all have to survive it.
  it('keeps every spelling of a plain color the CSS parser understands', () => {
    expect(sanitizeReaderColorStyle('color: RED')).toBe('color: red');
    expect(sanitizeReaderColorStyle('color: #abc')).toBe('color: rgb(170, 187, 204)');
    expect(sanitizeReaderColorStyle('color: rgb(1, 2, 3)')).toBe('color: rgb(1, 2, 3)');
    expect(sanitizeReaderColorStyle('color: rgba(1, 2, 3, 0.5)')).toBe('color: rgba(1, 2, 3, 0.5)');
    expect(sanitizeReaderColorStyle('COLOR: purple')).toBe('color: purple');
  });

  // A later unparseable value must not take the earlier one down with it,
  // otherwise one typo further along the attribute silently drops the color.
  it('keeps the last color the parser accepted, not the last one written', () => {
    expect(sanitizeReaderColorStyle('color: purple; color: bogus')).toBe('color: purple');
    expect(sanitizeReaderColorStyle('color: purple; color: var(--secret)')).toBe('color: purple');
  });
});

describe('sanitizeReaderHtml', () => {
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
