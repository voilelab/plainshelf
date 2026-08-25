import type { MarkdownChapterHeading } from '@/utils/markdownChapters';

export interface SourceTextEdit {
  start: number;
  end: number;
  replacement: string;
}

export function insertChapterEdit(content: string, offset: number): SourceTextEdit {
  const start = Math.max(0, Math.min(content.length, offset));
  const eol = content.match(/\r?\n/)?.[0] === '\r\n' ? '\r\n' : '\n';
  const needsLeadingBlank = start > 0 && !content.slice(0, start).endsWith(`${eol}${eol}`);
  return {
    start,
    end: start,
    replacement: `${needsLeadingBlank ? eol : ''}## Untitled chapter${eol}${eol}`
  };
}

export function renameChapterEdit(
  content: string,
  heading: MarkdownChapterHeading,
  title: string
): SourceTextEdit {
  const hadCR = content.slice(heading.endOffset - 1, heading.endOffset) === '\r';
  return {
    start: heading.startOffset,
    end: heading.endOffset,
    replacement: `## ${title.trim()}${hadCR ? '\r' : ''}`
  };
}

export function mergeChapterEdit(
  content: string,
  heading: MarkdownChapterHeading
): SourceTextEdit {
  let end = heading.endOffset;
  if (content.slice(end, end + 1) === '\n') end += 1;
  return {
    start: heading.startOffset,
    end,
    replacement: ''
  };
}
