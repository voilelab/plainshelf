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
  if (!query) return { value: text, occurrences: 0 };

  const parts = text.split(query);
  const occurrences = parts.length - 1;
  return {
    value: occurrences > 0 ? parts.join(replacement) : text,
    occurrences
  };
}

/** Maps a UTF-16 offset through a non-overlapping replace-all operation. */
export function mapOffsetThroughReplaceAll(
  text: string,
  query: string,
  replacement: string,
  offset: number,
  affinity: 'forward' | 'backward' = 'forward'
): number {
  const clamped = clampTextOffset(text, offset);
  if (!query) return clamped;

  let delta = 0;
  for (const match of matchOffsets(text, query)) {
    const end = match + query.length;
    if (clamped < match || (clamped === match && affinity === 'backward')) break;
    if (clamped > end || (clamped === end && affinity === 'forward')) {
      delta += replacement.length - query.length;
      continue;
    }
    return match + delta + (affinity === 'backward' ? 0 : replacement.length);
  }
  return clamped + delta;
}

/** Maps an offset through one range replacement. */
export function mapOffsetThroughReplacement(
  offset: number,
  startOffset: number,
  endOffset: number,
  replacementLength: number,
  affinity: 'forward' | 'backward' = 'forward'
): number {
  const start = Math.max(0, startOffset);
  const end = Math.max(start, endOffset);
  if (offset < start || (offset === start && affinity === 'backward')) return offset;
  if (offset > end || (offset === end && affinity === 'forward')) {
    return offset + replacementLength - (end - start);
  }
  return start + (affinity === 'backward' ? 0 : replacementLength);
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
