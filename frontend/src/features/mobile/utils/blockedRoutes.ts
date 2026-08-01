/**
 * Routes the mobile shell refuses to open.
 *
 * The Android client is a read-only reading client, so pages that exist purely
 * to edit the shelf are unreachable there. The API client rejects the writes
 * regardless (see api/client.ts); this keeps the pages themselves off the
 * device rather than rendering a form whose submit can only fail.
 *
 * Kept out of router.ts so it can be imported without constructing a router
 * (createWebHistory needs a real `window`).
 */
export const MOBILE_BLOCKED_ROUTES = new Set([
  'book-edit',
  'book-sources-edit',
  'trash',
  'duplicate-content',
  'maintenance-missing-author',
  'maintenance-missing-cover',
  'maintenance-missing-language',
  'maintenance-low-char-count'
]);
