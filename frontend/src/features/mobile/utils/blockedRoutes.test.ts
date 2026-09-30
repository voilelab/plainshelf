import { describe, expect, it } from 'vitest';

import { MOBILE_BLOCKED_ROUTES, stripMobileBlockedQuery } from './blockedRoutes';

// The guard wiring itself needs a mounted router with a live mobile runtime,
// which the e2e mobile suite covers. This pins the policy: which named routes
// the mobile shell refuses, and — just as importantly — which it must keep.
// The metadata editor is absent from both lists: it has no route of its own,
// only a modal on `book-detail`, which the shell keeps and renders read-only.
describe('MOBILE_BLOCKED_ROUTES', () => {
  it.each([
    'book-sources-edit',
    'admin-logs',
    'trash',
    'duplicate-content',
    'similar-content'
  ])('blocks %s', (name) => {
    expect(MOBILE_BLOCKED_ROUTES.has(name)).toBe(true);
  });

  it.each([
    'home',
    'library',
    'book-detail',
    'reader',
    'read-history',
    'downloads',
    'settings',
    'mobile-shelves',
    'mobile-shelf-add',
    'mobile-shelf-edit',
    // Now read-only book-list filters that redirect through `library`, so the
    // mobile shell must no longer refuse them by name.
    'maintenance-missing-author',
    'maintenance-missing-cover',
    'maintenance-missing-language'
  ])('keeps %s reachable', (name) => {
    expect(MOBILE_BLOCKED_ROUTES.has(name)).toBe(false);
  });
});

// The other half of the policy: a write surface reachable by query rather than
// by route name. `library` must stay reachable, so the guard cannot refuse the
// navigation outright — it has to clean it.
describe('stripMobileBlockedQuery', () => {
  it('removes the import query', () => {
    expect(stripMobileBlockedQuery({ import: '1' })).toEqual({});
  });

  it('keeps unrelated parameters, including the mobile preview flag', () => {
    expect(
      stripMobileBlockedQuery({ import: '1', 'mobile-shell-preview': '1', page: '2' })
    ).toEqual({ 'mobile-shell-preview': '1', page: '2' });
  });

  // Null is what stops the guard redirecting to a location that still matches.
  it('reports nothing to strip so the guard does not loop', () => {
    expect(stripMobileBlockedQuery({})).toBeNull();
    expect(stripMobileBlockedQuery({ page: '2' })).toBeNull();
    expect(stripMobileBlockedQuery(stripMobileBlockedQuery({ import: '1' })!)).toBeNull();
  });

  it('does not mutate the query it was given', () => {
    const query = { import: '1' };
    stripMobileBlockedQuery(query);
    expect(query).toEqual({ import: '1' });
  });
});
