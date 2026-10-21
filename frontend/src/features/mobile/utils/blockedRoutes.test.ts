import { describe, expect, it } from 'vitest';

import { MOBILE_BLOCKED_ROUTES, stripMobileBlockedQuery } from './blockedRoutes';

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
    'maintenance-missing-author',
    'maintenance-missing-cover',
    'maintenance-missing-language'
  ])('keeps %s reachable', (name) => {
    expect(MOBILE_BLOCKED_ROUTES.has(name)).toBe(false);
  });
});

describe('stripMobileBlockedQuery', () => {
  it('removes the import query', () => {
    expect(stripMobileBlockedQuery({ import: '1' })).toEqual({});
  });

  it('keeps unrelated parameters, including the mobile preview flag', () => {
    expect(
      stripMobileBlockedQuery({ import: '1', 'mobile-shell-preview': '1', page: '2' })
    ).toEqual({ 'mobile-shell-preview': '1', page: '2' });
  });

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
