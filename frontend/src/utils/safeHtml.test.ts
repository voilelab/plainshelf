// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { toPlainSummary } from './safeHtml';

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
