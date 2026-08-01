import { describe, expect, it } from 'vitest';

import { MOBILE_BLOCKED_ROUTES } from './blockedRoutes';

// The guard wiring itself needs a mounted router with a live mobile runtime,
// which the e2e mobile suite covers. This pins the policy: which named routes
// the mobile shell refuses, and — just as importantly — which it must keep.
describe('MOBILE_BLOCKED_ROUTES', () => {
  it.each([
    'book-edit',
    'book-sources-edit',
    'trash',
    'duplicate-content',
    'maintenance-missing-author',
    'maintenance-missing-cover',
    'maintenance-missing-language',
    'maintenance-low-char-count'
  ])('blocks %s', (name) => {
    expect(MOBILE_BLOCKED_ROUTES.has(name)).toBe(true);
  });

  it.each([
    'dashboard',
    'library',
    'book-detail',
    'reader',
    'read-history',
    'downloads',
    'settings',
    'mobile-connect'
  ])('keeps %s reachable', (name) => {
    expect(MOBILE_BLOCKED_ROUTES.has(name)).toBe(false);
  });
});
