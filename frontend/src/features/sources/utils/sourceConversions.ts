import {
  parseMarkdownHeadingLine,
  updateMarkdownFenceState,
  type MarkdownFenceState
} from '@/utils/markdownLineSyntax';

function stripInlineMarkdown(value: string): string {
  return value
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*([^*]*)\*\*/g, '$1')
    .replace(/__([^_]*)__/g, '$1')
    .replace(/\*([^*]*)\*/g, '$1')
    .replace(/_([^_]*)_/g, '$1');
}

export function markdownToPlainText(content: string): string {
  let fence: MarkdownFenceState | null = null;
  return content.split(/\r?\n/).map((line) => {
    const transition = updateMarkdownFenceState(line, fence);
    if (transition.boundary) {
      fence = transition.state;
      return '';
    }
    if (fence) return line;
    if (/^\s*(?:-{3,}|\*{3,})\s*$/.test(line)) return '';
    const image = /^\s*!\[([^\]]*)\]\([^)]*\)\s*$/.exec(line);
    if (image) return image[1];
    const heading = parseMarkdownHeadingLine(line);
    return stripInlineMarkdown(heading?.title ?? line);
  }).join('\n').replace(/\n{3,}/g, '\n\n').trim();
}

/**
 * The default chapter-heading pattern, shared by the regex converter and the
 * post-import chapter detector so both agree on what a chapter line is.
 *
 * It matches Chinese web-novel headings — `第…章/回/節/卷/部/篇` with Arabic,
 * full-width, or Chinese numerals — and English `Chapter N` lines, so a Chinese
 * or English TXT detects chapters out of the box rather than needing the reader
 * to know the source-conversion feature exists.
 */
export const DEFAULT_CHAPTER_PATTERN =
  '^\\s*(第[0-9０-９零一二三四五六七八九十百千兩]+[章回節卷部篇].*|Chapter\\s+.+)$';

/** Counts the lines that read as chapter headings under `pattern`. */
export function countTextChapters(
  content: string,
  pattern: string = DEFAULT_CHAPTER_PATTERN
): number {
  const regex = new RegExp(pattern);
  let chapters = 0;
  for (const line of content.split(/\r?\n/)) {
    if (regex.test(line)) {
      chapters += 1;
    }
  }
  return chapters;
}

export function textToMarkdownByRegex(content: string, pattern: string): { content: string; chapters: number } {
  const regex = new RegExp(pattern);
  let chapters = 0;
  const converted = content.split(/\r?\n/).map((line) => {
    regex.lastIndex = 0;
    const match = regex.exec(line);
    if (!match) return line;
    chapters += 1;
    const title = String(match[1] ?? match[0] ?? line).replace(/^#{1,6}\s+/, '').trim();
    return `## ${title || `Part ${chapters}`}`;
  }).join('\n');
  return { content: converted, chapters };
}

export function textToMarkdownByLineCount(content: string, lineCount: number): { content: string; chapters: number } {
  const size = Math.max(1, Math.trunc(lineCount));
  const lines = content.split(/\r?\n/);
  const output: string[] = [];
  let chapters = 0;
  for (let index = 0; index < lines.length; index += 1) {
    if (index % size === 0) {
      chapters += 1;
      output.push(`## Part ${chapters}`, '');
    }
    output.push(lines[index]);
  }
  return { content: output.join('\n'), chapters };
}
