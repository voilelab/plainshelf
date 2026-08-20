/**
 * Reads the reader's `?section=` deep link, which carries a section index —
 * the reader's own, not a chapter number — so it survives a chapter rename.
 * Anything that is not a whole number is treated as no link at all; a number
 * outside the book's range is left to the reader to clamp.
 */
export function parseSectionQuery(raw: unknown): number | null {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  if (trimmed === '') {
    return null;
  }

  const parsed = Number(trimmed);
  return Number.isInteger(parsed) ? parsed : null;
}
