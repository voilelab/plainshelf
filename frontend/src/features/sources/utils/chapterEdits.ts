import type { MarkdownChapterHeading } from '@/features/reader/utils/markdownChapters';

export interface SourceTextEdit {
  start: number;
  end: number;
  replacement: string;
}

export function insertChapterEdit(content: string, offset: number): SourceTextEdit {
  const start = Math.max(0, Math.min(content.length, offset));
  const needsLeadingBlank = start > 0 && !content.slice(0, start).endsWith('\n\n');
  return {
    start,
    end: start,
    replacement: `${needsLeadingBlank ? '\n' : ''}## Untitled chapter\n\n`
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
