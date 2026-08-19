import { describe, expect, it } from 'vitest';
import {
  markdownToPlainText,
  textToMarkdownByLineCount,
  textToMarkdownByRegex
} from './sourceConversions';

describe('source conversions', () => {
  it('converts Markdown to readable plain text', () => {
    expect(markdownToPlainText('## Chapter\n**Bold** and `code`\n---\n![Map](assets/map.png)\n```\nx < y\n```'))
      .toBe('Chapter\nBold and code\n\nMap\n\nx < y');
  });

  it('converts matching title lines to H2', () => {
    expect(textToMarkdownByRegex('Chapter 1\nText\nChapter 2', '^(Chapter \\d+)$')).toEqual({
      content: '## Chapter 1\nText\n## Chapter 2', chapters: 2
    });
  });

  it('inserts stable headings at fixed line boundaries', () => {
    expect(textToMarkdownByLineCount('a\nb\nc', 2)).toEqual({
      content: '## Part 1\n\na\nb\n## Part 2\n\nc', chapters: 2
    });
  });

});
