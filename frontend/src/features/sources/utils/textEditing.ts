export interface TextReplacement {
  value: string;
  selectionStart: number;
  selectionEnd: number;
}

export interface ReplaceAllResult {
  value: string;
  occurrences: number;
}

export function clampTextOffset(text: string, offset: number): number {
  return Math.max(0, Math.min(text.length, offset));
}

export function matchOffsets(text: string, query: string): number[] {
  if (!query) return [];

  const offsets: number[] = [];
  let offset = 0;
  while (offset <= text.length - query.length) {
    const index = text.indexOf(query, offset);
    if (index === -1) break;
    offsets.push(index);
    offset = index + query.length;
  }
  return offsets;
}

export function findMatchOffset(
  text: string,
  query: string,
  selectionStart: number,
  selectionEnd: number,
  backward: boolean
): number | null {
  if (!query) return null;

  const start = clampTextOffset(text, selectionStart);
  const end = clampTextOffset(text, selectionEnd);
  let index = backward
    ? text.slice(0, start).lastIndexOf(query)
    : text.indexOf(query, end);

  if (index === -1) {
    index = backward ? text.lastIndexOf(query) : text.indexOf(query);
  }
  return index === -1 ? null : index;
}

export function replaceTextRange(
  text: string,
  startOffset: number,
  endOffset: number,
  replacement: string
): TextReplacement {
  const start = clampTextOffset(text, startOffset);
  const end = Math.max(start, clampTextOffset(text, endOffset));
  return {
    value: `${text.slice(0, start)}${replacement}${text.slice(end)}`,
    selectionStart: start,
    selectionEnd: start + replacement.length
  };
}

export function replaceAllText(text: string, query: string, replacement: string): ReplaceAllResult {
  const occurrences = matchOffsets(text, query).length;
  return {
    value: occurrences > 0 ? text.split(query).join(replacement) : text,
    occurrences
  };
}

export function paragraphStartOffset(text: string, cursorOffset: number): number {
  const before = text.slice(0, clampTextOffset(text, cursorOffset));
  const blankLine = /(?:^|\n)[ \t]*\r?\n/g;
  let start = 0;
  for (const match of before.matchAll(blankLine)) {
    start = (match.index ?? 0) + match[0].length;
  }
  return start;
}
