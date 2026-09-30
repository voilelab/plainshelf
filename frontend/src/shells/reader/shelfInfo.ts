import type { ShelfInfo } from '@/api/shelves';
import { readerBootConfig } from './bootConfig';

/**
 * The one shelf the reader app reports, synthesized from its boot config.
 *
 * There is no shelf behind a single book package, but the app addresses books
 * as /api/shelves/{shelf}/books/{book} whatever it is talking to. Answering
 * here keeps ReaderLayout's active-shelf gate and the shared provider working
 * unchanged, and means the reader never issues a shelf-list request that its
 * own backend would have to invent an answer for.
 */
export function activeReaderShelfInfo(): ShelfInfo | null {
  const boot = readerBootConfig();
  if (!boot?.shelf_id) {
    return null;
  }

  // Not read-only: the reader edits the package it opened, and the one thing
  // it cannot write is decided by the package itself, not by a shelf setting.
  return { id: boot.shelf_id, name: 'Book', readOnly: false };
}
