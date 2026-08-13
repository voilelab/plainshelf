import { describe, expect, it } from 'vitest';
import {
  markdownToPlainText,
  textToMarkdownByLineCount,
  textToMarkdownByRegex,
  upgradeLegacyToMarkdown
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

  it('upgrades legacy boundaries without changing the original input', () => {
    expect(upgradeLegacyToMarkdown('a\nb\nc', { type: 'boundary', boundaries: [1, 3] }))
      .toBe('## Part 1\n\na\nb\n## Part 2\n\nc');
    expect(upgradeLegacyToMarkdown('## Existing\nText', { type: 'line_count', line_count: 1 }))
      .toBe('## Existing\nText');
  });

  it('turns legacy regex title lines into H2 titles', () => {
    expect(upgradeLegacyToMarkdown('Chapter 1\nText\nChapter 2', { type: 'regex', regex: '^Chapter \\d+$' }))
      .toBe('## Chapter 1\nText\n## Chapter 2');
  });

  it('does not mistake an H2-looking code example for existing chapter structure', () => {
    const content = '```md\n## example only\n```\nBody';
    expect(upgradeLegacyToMarkdown(content, { type: 'none' }))
      .toBe(`## Part 1\n\n${content}`);
  });
});
