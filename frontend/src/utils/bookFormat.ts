import type { BookFormat } from '@/types/book';

export function resolveReadingFormat(
  sourceFormat: string | undefined,
  bookFormat: string | undefined
): BookFormat {
  if (sourceFormat === 'md' || sourceFormat === 'txt' || sourceFormat === 'images') {
    return sourceFormat;
  }

  return bookFormat === 'md' || bookFormat === 'images' ? bookFormat : 'txt';
}
