import type { BookFormat } from '@/types/book';

export function resolveReadingFormat(
  sourceFormat: string | undefined,
  bookFormat: string | undefined
): BookFormat {
  if (sourceFormat === 'md' || sourceFormat === 'txt') {
    return sourceFormat;
  }

  return bookFormat === 'md' ? 'md' : 'txt';
}
