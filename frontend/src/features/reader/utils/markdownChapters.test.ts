import { describe, expect, it } from 'vitest';
import {
  buildMarkdownEditorSections,
  buildMarkdownH2Sections,
  findMarkdownEditorSection
} from './markdownChapters';

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

  it('only closes a fence with the same marker and a sufficient run length', () => {
    const content = [
      '````md',
      '~~~',
      '```',
      '## Still code',
      '````',
      '## Real chapter',
      'Body'
    ].join('\n');
    const sections = buildMarkdownH2Sections(content);
    expect(sections.map((section) => section.title)).toEqual(['Opening', 'Real chapter']);
  });

  it('does not treat four-space-indented code as a heading', () => {
    const sections = buildMarkdownH2Sections('    ## code example\n\n## Real\nBody');
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

  it('builds selectable editor ranges for opening and duplicate chapter titles', () => {
    const content = '# Book\r\nIntro 😀\r\n## Same\r\nOne\r\n## Same\r\nTwo';
    const sections = buildMarkdownEditorSections(content);
    expect(sections.map(({ kind, title }) => ({ kind, title }))).toEqual([
      { kind: 'opening', title: 'Book' },
      { kind: 'chapter', title: 'Same' },
      { kind: 'chapter', title: 'Same' }
    ]);
    expect(sections[1].startOffset).toBe(content.indexOf('## Same'));
    expect(sections[1].endOffset).toBe(content.lastIndexOf('## Same'));
    expect(sections[2].headingIndex).toBe(1);
  });

  it('keeps fenced H2 text inside one editor section', () => {
    const content = '## One\n```md\n## Code\n```\nBody';
    const sections = buildMarkdownEditorSections(content);
    expect(sections).toHaveLength(1);
    expect(sections[0]).toMatchObject({ kind: 'chapter', startOffset: 0, endOffset: content.length });
  });

  it('uses affinity to resolve a cursor exactly between chapters', () => {
    const content = '## One\nBody\n## Two\nMore';
    const sections = buildMarkdownEditorSections(content);
    const boundary = content.indexOf('## Two');
    expect(findMarkdownEditorSection(sections, boundary, 'forward')?.title).toBe('Two');
    expect(findMarkdownEditorSection(sections, boundary, 'backward')?.title).toBe('One');
    expect(findMarkdownEditorSection(sections, content.length, 'forward')?.title).toBe('Two');
  });
});
