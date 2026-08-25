import { describe, expect, it } from 'vitest';
import { scanMarkdownH2Headings } from '@/utils/markdownChapters';
import { insertChapterEdit, mergeChapterEdit, renameChapterEdit } from './chapterEdits';

describe('source chapter edits', () => {
  it('inserts a chapter at the document start and after prose', () => {
    expect(insertChapterEdit('', 0)).toEqual({
      start: 0,
      end: 0,
      replacement: '## Untitled chapter\n\n'
    });
    expect(insertChapterEdit('Opening text\n', 13).replacement).toBe('\n## Untitled chapter\n\n');
    expect(insertChapterEdit('Opening text\n\n', 14).replacement).toBe('## Untitled chapter\n\n');
  });

  it('uses CRLF consistently when inserting into a CRLF source', () => {
    expect(insertChapterEdit('Opening text\r\n', 14).replacement).toBe(
      '\r\n## Untitled chapter\r\n\r\n'
    );
    expect(insertChapterEdit('Opening text\r\n\r\n', 16).replacement).toBe(
      '## Untitled chapter\r\n\r\n'
    );
  });

  it('renames a CRLF heading without changing its line ending', () => {
    const content = '## Old title\r\nBody';
    const heading = scanMarkdownH2Headings(content)[0];
    expect(renameChapterEdit(content, heading, ' New title ')).toEqual({
      start: 0,
      end: 13,
      replacement: '## New title\r'
    });
  });

  it('merges a chapter by removing its heading and adjacent newline', () => {
    const content = 'Opening\n\n## Chapter\nBody';
    const heading = scanMarkdownH2Headings(content)[0];
    const edit = mergeChapterEdit(content, heading);
    expect(edit).toEqual({ start: 9, end: 20, replacement: '' });
    expect(`${content.slice(0, edit.start)}${edit.replacement}${content.slice(edit.end)}`).toBe('Opening\n\nBody');
  });
});
