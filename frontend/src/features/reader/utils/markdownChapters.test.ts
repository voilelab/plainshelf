import { describe, expect, it } from 'vitest';
import { buildMarkdownH2Sections } from './markdownChapters';

describe('buildMarkdownH2Sections', () => {
  it('uses H2 headings and keeps headings in section text', () => {
    const sections = buildMarkdownH2Sections('## One\nBody\n---\n## Two\nMore');
    expect(sections.map((section) => section.title)).toEqual(['One', 'Two']);
    expect(sections[0].text).toBe('## One\nBody\n---\n');
    expect(sections[1].text).toBe('## Two\nMore');
  });

  it('creates an opening section named by the first H1', () => {
    const sections = buildMarkdownH2Sections('# My Book\nPreface\n\n## Chapter\nText');
    expect(sections.map((section) => section.title)).toEqual(['My Book', 'Chapter']);
    expect(sections[0]).toMatchObject({ startOffset: 0, endOffset: '# My Book\nPreface\n\n'.length });
  });

  it('ignores H2 inside fences plus H3 and horizontal rules', () => {
    const content = '```md\n## Not a chapter\n```\n### Detail\n***\n## Real\nText';
    const sections = buildMarkdownH2Sections(content);
    expect(sections.map((section) => section.title)).toEqual(['Opening', 'Real']);
  });

  it('preserves CRLF and UTF-16 offsets', () => {
    const content = '# 📚\r\nIntro 😀\r\n## 第二章\r\nText';
    const sections = buildMarkdownH2Sections(content);
    const headingOffset = content.indexOf('## 第二章');
    expect(sections[0].endOffset).toBe(headingOffset);
    expect(sections[1].startOffset).toBe(headingOffset);
    expect(content.slice(sections[1].startOffset)).toBe('## 第二章\r\nText');
  });

  it('returns one section for Markdown without H2', () => {
    expect(buildMarkdownH2Sections('# Title\nText')).toEqual([
      { index: 0, startOffset: 0, endOffset: 12, title: 'Title', text: '# Title\nText' }
    ]);
  });
});
