import type { BookFormat } from '@/types/book';

/**
 * The format a book's text is parsed as. A source's own metadata is
 * authoritative for every source this build writes, so it wins over the
 * book-level format, which an older or hand-edited shelf may not have kept in
 * step. Anything else reads as plain text.
 */
export function resolveReadingFormat(
  sourceFormat: string | undefined,
  bookFormat: string | undefined
): BookFormat {
  if (sourceFormat === 'md' || sourceFormat === 'txt') {
    return sourceFormat;
  }

  return bookFormat === 'md' ? 'md' : 'txt';
}
