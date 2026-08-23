import type { DownloadState } from '@/types/book';

/**
 * The mobile shell requires a book to be downloaded to the device before it can
 * be read: the reader route is refused for anything else and the user is sent
 * back to the book's detail page to download it first.
 *
 * Kept out of routerGuard.ts so the decision is unit-testable without a router
 * or a live provider, and out of any shared (non-mobile) module so it stays on
 * the mobile side of the bundle boundary.
 */

/**
 * Query flag the guard adds to the book-detail redirect so the detail page can
 * explain why the reader did not open.
 *
 * BookDetailPage reads the same literal (`downloadRequired`) directly — it is a
 * shared, non-mobile module and cannot import this mobile-only file, matching
 * how it already reads `imported` / `saved` / `copied` as inline literals.
 */
export const DOWNLOAD_REQUIRED_QUERY_KEY = 'downloadRequired';

// `update_available` means a downloaded copy exists that is merely stale, so it
// is still readable; only these two states satisfy the download requirement.
const READABLE_STATES: ReadonlySet<DownloadState> = new Set<DownloadState>([
  'downloaded',
  'update_available'
]);

/**
 * Whether a book in the given download state may be opened in the reader.
 */
export function isReadableWhenDownloadRequired(state: DownloadState): boolean {
  return READABLE_STATES.has(state);
}
