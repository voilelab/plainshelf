export function clampTextOffset(text: string, offset: number): number {
  return Math.max(0, Math.min(text.length, offset));
}

/**
 * The offset the paragraph containing `cursorOffset` starts at.
 *
 * Used to decide where an inserted chapter heading goes, so that adding a
 * chapter from inside a paragraph does not split it.
 */
export function paragraphStartOffset(text: string, cursorOffset: number): number {
  const before = text.slice(0, clampTextOffset(text, cursorOffset));
  const blankLine = /(?:^|\n)[ \t]*\r?\n/g;
  let start = 0;
  for (const match of before.matchAll(blankLine)) {
    start = (match.index ?? 0) + match[0].length;
  }
  return start;
}
