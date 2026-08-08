/**
 * Routes the mobile shell refuses to open.
 *
 * The Android client is a read-only reading client, so shelf-editing pages and
 * server administration logs are unreachable there. The API client rejects
 * writes regardless (see api/client.ts); this also keeps unsupported pages off
 * the device rather than rendering controls that cannot be used.
 *
 * Kept out of router.ts so it can be imported without constructing a router
 * (createWebHistory needs a real `window`).
 */
export const MOBILE_BLOCKED_ROUTES = new Set([
  'book-edit',
  'book-sources-edit',
  'admin-logs',
  'trash',
  'duplicate-content',
  'maintenance-missing-author',
  'maintenance-missing-cover',
  'maintenance-missing-language',
  'maintenance-low-char-count'
]);
