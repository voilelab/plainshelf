export const MOBILE_BLOCKED_ROUTES = new Set([
  'book-sources-edit',
  'admin-logs',
  'trash',
  'duplicate-content',
  'similar-content'
]);

// Vue Router 5.3.1 resolves redirects before running navigation guards (pushWithRedirect).
const MOBILE_BLOCKED_QUERY_KEYS = ['import'];

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
