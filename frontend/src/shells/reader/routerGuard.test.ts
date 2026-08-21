import { describe, expect, it } from 'vitest';
import type { Router } from 'vue-router';

import { READER_BLOCKED_ROUTES, stripReaderBlockedQuery } from './blockedRoutes';
import { installReaderRouterGuards } from './routerGuard';

type Guard = (to: {
  name?: string;
  path: string;
  query: Record<string, string>;
  hash: string;
}) => unknown;

/**
 * Installs the guard against a stand-in router and hands back what it
 * registered. The policy is what these tests are about, so driving the callback
 * directly keeps them independent of the shared route table.
 */
function readerGuard(): Guard {
  let guard: Guard | undefined;
  const router = {
    beforeEach(fn: Guard) {
      guard = fn;
    }
  } as unknown as Router;

  installReaderRouterGuards(router);
  if (!guard) {
    throw new Error('installReaderRouterGuards registered no guard');
  }
  return guard;
}

function navigateTo(name: string, query: Record<string, string> = {}) {
  return readerGuard()({ name, path: `/${name}`, query, hash: '' });
}

describe('installReaderRouterGuards', () => {
  it.each([...READER_BLOCKED_ROUTES])('sends %s back to the library', (name) => {
    expect(navigateTo(name)).toEqual({ name: 'library' });
  });

  // The pages a reading server does serve stay reachable, including the reader
  // itself and the settings page, whose remaining tabs are device-local.
  it.each(['library', 'dashboard', 'book-detail', 'reader', 'read-history', 'settings'])(
    'lets %s through',
    (name) => {
      expect(navigateTo(name)).toBe(true);
    }
  );

  // /import is a route-level redirect to /books?import=1, which vue-router
  // resolves before the guard runs, so the query is the only thing left to
  // refuse.
  it('strips the query that opens the import modal', () => {
    expect(navigateTo('library', { import: '1', page: '2' })).toEqual({
      path: '/library',
      query: { page: '2' },
      hash: '',
      replace: true
    });
  });

  // A guard that redirected unconditionally would redirect to a location that
  // still matches, and loop.
  it('does not redirect a query that is already clean', () => {
    expect(navigateTo('library', { page: '2' })).toBe(true);
  });

  it('ignores a route with no name', () => {
    expect(readerGuard()({ path: '/whatever', query: {}, hash: '' })).toBe(true);
  });
});

describe('stripReaderBlockedQuery', () => {
  it('returns null when there is nothing to strip', () => {
    expect(stripReaderBlockedQuery({ page: '1' })).toBeNull();
  });

  it('leaves the original query alone', () => {
    const query = { import: '1', page: '1' };

    expect(stripReaderBlockedQuery(query)).toEqual({ page: '1' });
    expect(query).toEqual({ import: '1', page: '1' });
  });
});
