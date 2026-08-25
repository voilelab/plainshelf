import { describe, expect, it } from 'vitest';
import {
  buildMarkdownChapterList,
  buildMarkdownEditorSections,
  buildMarkdownH2Sections,
  findMarkdownEditorSection,
  scanMarkdownH2Headings
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
    const content = '# 📚\r\nIntro 🍥\r\n## 第二章\r\nText';
    const sections = buildMarkdownH2Sections(content);
    const headingOffset = content.indexOf('## 第二章');
    expect(sections[0].endOffset).toBe(headingOffset);
    expect(sections[1].startOffset).toBe(headingOffset);
    expect(content.slice(sections[1].startOffset)).toBe('## 第二章\r\nText');
  });

  // The visible title is what the chapter list shows, so inline markup has to
  // be resolved to its text rather than shown as source or dropped entirely.
  it('shows heading markup as its rendered text', () => {
    const sections = buildMarkdownH2Sections('## **Bold** and `code` and *italic*\nBody');
    expect(sections[0].title).toBe('Bold and code and italic');
  });

  it('names a chapter whose heading is only markup', () => {
    expect(buildMarkdownH2Sections('## ``\nBody')[0].title).toBe('Untitled chapter');
  });

  it('leaves no padding behind when it removes inline markup', () => {
    expect(buildMarkdownH2Sections('## `  code  `\nBody')[0].title).toBe('code');
  });

  // The editor replaces a heading through this range, so it has to cover the
  // whole heading line, including the last line of a document that ends
  // without a newline.
  it('reports the heading line range including an unterminated last line', () => {
    const content = '## One\nBody\n## Last';
    expect(scanMarkdownH2Headings(content)).toEqual([
      { startOffset: 0, endOffset: 6, title: 'One' },
      { startOffset: content.indexOf('## Last'), endOffset: content.length, title: 'Last' }
    ]);
  });

  it('reads a heading on a last line that has no newline', () => {
    expect(buildMarkdownH2Sections('# Title')[0].title).toBe('Title');
  });

  // Reading progress is stored as an offset into the whole document, so a
  // chapter range that is short by even one code unit resumes on the wrong line.
  it('keeps chapter ranges exact when a fenced block precedes the heading', () => {
    const content = '```md\n## Not a chapter\n```\n## Real\nBody';
    const sections = buildMarkdownH2Sections(content);
    expect(sections[1].startOffset).toBe(content.indexOf('## Real'));
    expect(sections[1].text).toBe('## Real\nBody');
  });

  it('keeps the last chapter reaching the end of a document with no trailing newline', () => {
    const content = '## One\nBody\n## Last';
    const sections = buildMarkdownH2Sections(content);
    expect(sections[1].endOffset).toBe(content.length);
    expect(sections[1].text).toBe('## Last');
  });

  it('returns one section for Markdown without H2', () => {
    expect(buildMarkdownH2Sections('# Title\nText')).toEqual([
      { index: 0, startOffset: 0, endOffset: 12, title: 'Title', text: '# Title\nText' }
    ]);
  });

  // "Part 1" is the last resort for a document that names itself nowhere.
  it('names a document that has neither an H2 nor an H1', () => {
    expect(buildMarkdownH2Sections('Just prose')).toEqual([
      { index: 0, startOffset: 0, endOffset: 10, title: 'Part 1', text: 'Just prose' }
    ]);
    expect(buildMarkdownEditorSections('Just prose')[0]).toMatchObject({ kind: 'opening' });
  });

  // The opening section is the text before the first chapter. Only an H1 in
  // that range names it, and blank lines alone are not an opening at all.
  it('takes the opening title from an H1 that precedes the first chapter', () => {
    const later = buildMarkdownEditorSections('## Chapter\nBody\n# Late title\nMore');
    expect(later.map(({ kind, title }) => ({ kind, title })))
      .toEqual([{ kind: 'chapter', title: 'Chapter' }]);

    const fenced = buildMarkdownEditorSections('```md\n# Fake title\n```\n## Chapter\nBody');
    expect(fenced[0]).toMatchObject({ kind: 'opening', title: 'Opening' });

    // Same rule, with an opening section that exists and a fenced block in it:
    // the H1 after the chapter is still not the opening's title.
    const afterFence = buildMarkdownEditorSections(
      'Preface\n\n```markdown\ncode\n```\n\n## Chapter\nBody\n# Late title\nMore'
    );
    expect(afterFence[0]).toMatchObject({ kind: 'opening', title: 'Opening' });
  });

  it('gives leading blank lines to the first chapter instead of an empty opening', () => {
    const sections = buildMarkdownEditorSections('\n\n## One\nBody');
    expect(sections).toHaveLength(1);
    expect(sections[0]).toMatchObject({ kind: 'chapter', startOffset: 0 });
  });

  it('builds selectable editor ranges for opening and duplicate chapter titles', () => {
    const content = '# Book\r\nIntro 🍥\r\n## Same\r\nOne\r\n## Same\r\nTwo';
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

  // With only two chapters, "fall through to the last section" is
  // indistinguishable from "find the right one"; a middle chapter tells them
  // apart, and the editor selects the wrong chapter when they differ.
  it('resolves a cursor inside and at the start of a middle chapter', () => {
    const content = '## One\nBody\n## Two\nBody\n## Three\nBody';
    const sections = buildMarkdownEditorSections(content);
    const secondStart = content.indexOf('## Two');
    expect(findMarkdownEditorSection(sections, secondStart + 3, 'forward')?.title).toBe('Two');
    expect(findMarkdownEditorSection(sections, secondStart + 3, 'backward')?.title).toBe('Two');
    expect(findMarkdownEditorSection(sections, secondStart, 'forward')?.title).toBe('Two');
    expect(findMarkdownEditorSection(sections, secondStart, 'backward')?.title).toBe('One');
  });

  // Offset zero is where a freshly opened document parks its caret, and an
  // offset past the end arrives from stale reading progress.
  it('resolves the document boundaries without falling off either end', () => {
    const content = '## One\nBody\n## Two\nMore';
    const sections = buildMarkdownEditorSections(content);
    for (const affinity of ['forward', 'backward'] as const) {
      expect(findMarkdownEditorSection(sections, 0, affinity)?.title, affinity).toBe('One');
      expect(findMarkdownEditorSection(sections, -5, affinity)?.title, affinity).toBe('One');
    }
    expect(findMarkdownEditorSection(sections, content.length + 99, 'forward')?.title).toBe('Two');
    expect(findMarkdownEditorSection([], 0, 'forward')).toBeNull();
  });
});

describe('buildMarkdownChapterList', () => {
  // The detail page's list is a deep link into the reader, so an index that
  // does not name the same section the reader would open is a wrong jump.
  it('numbers chapters exactly as the reader sections do', () => {
    const content = '# Book\nPreface\n\n## One\nBody\n## Two\nMore';
    expect(buildMarkdownChapterList(content)).toEqual(
      buildMarkdownH2Sections(content).map((section) => ({
        index: section.index,
        title: section.title
      }))
    );
    expect(buildMarkdownChapterList(content).map((chapter) => chapter.title)).toEqual([
      'Book',
      'One',
      'Two'
    ]);
  });

  // Text with nothing to jump to must yield no list rather than a one-item one:
  // the detail page hides the whole card on an empty result.
  it('returns nothing when the text has no H2 to navigate to', () => {
    expect(buildMarkdownChapterList('Just prose, no headings at all.')).toEqual([]);
    expect(buildMarkdownChapterList('# Title\nBody\n### Detail\nMore')).toEqual([]);
    expect(buildMarkdownChapterList('```md\n## Not a chapter\n```\n')).toEqual([]);
    expect(buildMarkdownChapterList('')).toEqual([]);
  });

  it('keeps a single chapter listed', () => {
    expect(buildMarkdownChapterList('## Only\nBody')).toEqual([{ index: 0, title: 'Only' }]);
  });
});
