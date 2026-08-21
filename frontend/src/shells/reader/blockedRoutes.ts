/**
 * Routes the reader shell refuses to open.
 *
 * The standalone reading binary (cmd/plainshelf-read) mounts the reading routes
 * and nothing else, so these pages have no backend to talk to: they would render
 * their chrome and then fail every request against a 404. Refusing them here is
 * what keeps the reader an app that works rather than one whose sidebar is half
 * broken links.
 *
 * The same shape as the mobile shell's list (features/mobile/utils/
 * blockedRoutes.ts) and for a related reason, but not the same list and not
 * shared: mobile blocks what a read-only *client* cannot do, this blocks what a
 * reading *server* does not serve, and the two will drift. `downloads` is the
 * example — an offline cache is a mobile feature, not something a reader server
 * has an opinion about.
 *
 * Kept out of router.ts so it can be imported without constructing a router
 * (createWebHistory needs a real `window`).
 */
export const READER_BLOCKED_ROUTES = new Set([
  // Writes. The server refuses them; the pages should not be reachable either.
  'book-edit',
  'book-sources-edit',

  // Reads the reader server does not mount: /api/logs, the trash listing, and
  // the duplicate scan.
  'admin-logs',
  'trash',
  'duplicate-content',

  // The maintenance views exist to fix up a library rather than to read one.
  // They list books through routes a reader does serve, but every action they
  // offer is an edit, so they are a page with nothing to do here.
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
 * sees `library`, which has to stay reachable. The import modal then opens
 * purely off the query, from any link, deep link, or restored tab.
 */
export const READER_BLOCKED_QUERY_KEYS = ['import'];

/**
 * Strips the write-opening query parameters above, or returns null when there is
 * nothing to strip.
 *
 * Null rather than an unchanged copy so the caller can tell "already clean" from
 * "cleaned": a guard that redirected unconditionally would redirect to a
 * location that still matches, and loop.
 *
 * Typed structurally rather than against vue-router's LocationQuery so this
 * module stays importable without a router (see above).
 */
export function stripReaderBlockedQuery<T extends Record<string, unknown>>(query: T): T | null {
  const blocked = READER_BLOCKED_QUERY_KEYS.filter((key) => key in query);
  if (blocked.length === 0) {
    return null;
  }

  const stripped = { ...query };
  for (const key of blocked) {
    delete stripped[key];
  }
  return stripped;
}
