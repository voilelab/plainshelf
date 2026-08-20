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
