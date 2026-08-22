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
  'similar-content',
  'maintenance-missing-author',
  'maintenance-missing-cover',
  'maintenance-missing-language'
]);

/**
 * Query parameters that open a write surface on a route that is otherwise
 * readable, and so cannot be refused by route name.
 *
 * `/import` is a route-level redirect to `/books?import=1` (see router.ts), and
 * vue-router resolves a redirect before the guard runs — the guard only ever
 * sees `library`, which the mobile shell must keep reachable. The import modal
 * then opens purely off the query, from any link, deep link, or restored tab.
 */
export const MOBILE_BLOCKED_QUERY_KEYS = ['import'];

/**
 * Strips the write-opening query parameters above, or returns null when there
 * is nothing to strip.
 *
 * Null rather than an unchanged copy so the caller can tell "already clean"
 * from "cleaned": a guard that redirects unconditionally would redirect to a
 * location that still matches, and loop.
 *
 * Typed structurally rather than against vue-router's LocationQuery so this
 * module stays importable without a router (see above).
 */
export function stripMobileBlockedQuery<T extends Record<string, unknown>>(query: T): T | null {
  const blocked = MOBILE_BLOCKED_QUERY_KEYS.filter((key) => key in query);
  if (blocked.length === 0) {
    return null;
  }

  const stripped = { ...query };
  for (const key of blocked) {
    delete stripped[key];
  }
  return stripped;
}
