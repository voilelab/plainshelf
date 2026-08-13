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
