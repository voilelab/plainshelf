import { describe, expect, it } from 'vitest';
import {
  DEFAULT_CHAPTER_PATTERN,
  countTextChapters,
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

describe('default chapter detection', () => {
  it('matches Chinese and English chapter headings with the one default pattern', () => {
    const content = [
      '楔子',
      '第1回 開端',
      '尋常散文，只是引用了第一章這個詞',
      '第十二節 收束',
      'Chapter 3',
      '第三章'
    ].join('\n');

    // 第1回 / 第十二節 / Chapter 3 / 第三章 hit; the prose line that merely
    // mentions "第一章" mid-sentence does not, because the marker must open the line.
    expect(countTextChapters(content)).toBe(4);
  });

  it('previews more than zero chapters for a Chinese book with the default pattern', () => {
    const chineseBook = '第一章 風起\n正文……\n第二章 雲湧\n正文……';
    const converted = textToMarkdownByRegex(chineseBook, DEFAULT_CHAPTER_PATTERN);

    expect(converted.chapters).toBe(2);
    expect(converted.content).toContain('## 第一章 風起');
    expect(converted.content).toContain('## 第二章 雲湧');
  });

  it('detects nothing in ordinary prose that has no chapter lines', () => {
    expect(countTextChapters('一段沒有章節標記的散文。\n只是普通的文字。\nJust prose.')).toBe(0);
  });
});
