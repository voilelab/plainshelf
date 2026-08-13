import type { SplitConfig } from '@/types/book';
import { isMarkdownFenceLine, parseMarkdownHeadingLine } from '@/features/reader/utils/parseMarkdownBlocks';

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
  let inFence = false;
  return content.split(/\r?\n/).map((line) => {
    if (isMarkdownFenceLine(line)) {
      inFence = !inFence;
      return '';
    }
    if (inFence) return line;
    if (/^\s*(?:-{3,}|\*{3,})\s*$/.test(line)) return '';
    const image = /^\s*!\[([^\]]*)\]\([^)]*\)\s*$/.exec(line);
    if (image) return image[1];
    const heading = parseMarkdownHeadingLine(line);
    return stripInlineMarkdown(heading?.title ?? line);
  }).join('\n').replace(/\n{3,}/g, '\n\n').trim();
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

function lineStartOffsets(content: string): number[] {
  const starts = [0];
  for (let index = 0; index < content.length; index += 1) {
    if (content[index] === '\n') starts.push(index + 1);
  }
  return starts;
}

export function upgradeLegacyToMarkdown(content: string, config: SplitConfig): string {
  // Existing H2 Markdown already expresses the intended structure. Never
  // rewrite it merely because its source metadata is legacy.
  if (/^##\s+.+$/m.test(content)) return content;

  let offsets: number[] = [];
  if (config.type === 'regex' && config.regex?.trim()) {
    try {
      const regex = new RegExp(config.regex, 'gm');
      const lineRanges = Array.from(content.matchAll(regex), (match) => {
        const matchOffset = match.index ?? 0;
        const start = content.lastIndexOf('\n', Math.max(0, matchOffset - 1)) + 1;
        const newline = content.indexOf('\n', matchOffset);
        const end = newline === -1 ? content.length : newline;
        return { start, end };
      }).filter((range, index, all) => index === 0 || range.start !== all[index - 1].start);

      if (lineRanges.length > 0) {
        let upgraded = content;
        [...lineRanges].reverse().forEach(({ start, end }) => {
          const rawTitle = content.slice(start, end).replace(/\r$/, '').replace(/^#{1,6}\s+/, '').trim();
          const hadCR = content.slice(end - 1, end) === '\r';
          upgraded = `${upgraded.slice(0, start)}## ${rawTitle || 'Untitled chapter'}${hadCR ? '\r' : ''}${upgraded.slice(end)}`;
        });
        if (lineRanges[0].start > 0) {
          upgraded = `## Part 1\n\n${upgraded}`;
        }
        return upgraded;
      }
    } catch {
      offsets = [];
    }
  } else if (config.type === 'boundary') {
    const starts = lineStartOffsets(content);
    offsets = [0, ...(config.boundaries ?? [])
      .map((line) => starts[Math.trunc(line) - 1])
      .filter((offset): offset is number => offset !== undefined)];
  } else if (config.type === 'line_count' && (config.line_count ?? 0) > 0) {
    const starts = lineStartOffsets(content);
    for (let index = 0; index < starts.length; index += Math.trunc(config.line_count ?? 1)) {
      offsets.push(starts[index]);
    }
  }

  if (offsets.length === 0) return `## Part 1\n\n${content}`;
  let upgraded = content;
  [...new Set(offsets)].sort((a, b) => b - a).forEach((offset, reverseIndex, all) => {
    const part = all.length - reverseIndex;
    upgraded = `${upgraded.slice(0, offset)}## Part ${part}\n\n${upgraded.slice(offset)}`;
  });
  return upgraded;
}
