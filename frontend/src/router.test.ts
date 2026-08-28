// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router';

vi.mock('@/layouts/MainLayout.vue', () => ({ default: {} }));
vi.mock('@/layouts/ReaderLayout.vue', () => ({ default: {} }));

const { default: router } = await import('./router');

describe('legacy metadata editor route', () => {
  it('remains named and redirects to the detail modal query with replace', () => {
    const route = router.getRoutes().find((candidate) => candidate.name === 'book-edit');
    expect(route).toBeDefined();
    expect(typeof route?.redirect).toBe('function');

    const incoming = router.resolve('/books/book-1/edit?from=bookmark#details');
    const redirect = route?.redirect as (to: RouteLocationNormalized) => RouteLocationRaw;

    expect(redirect(incoming as unknown as RouteLocationNormalized)).toEqual({
      name: 'book-detail',
      params: { id: 'book-1' },
      query: { from: 'bookmark', edit: 'metadata' },
      hash: '#details',
      replace: true
    });
  });
});
