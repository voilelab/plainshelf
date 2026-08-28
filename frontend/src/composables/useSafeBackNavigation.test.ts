import { describe, expect, it, vi } from 'vitest';
import {
  createMemoryHistory,
  createRouter,
  type RouteRecordRaw,
  type Router
} from 'vue-router';
import {
  isSafePlainShelfBackTarget,
  navigateBackSafely
} from './useSafeBackNavigation';

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/home' },
  { path: '/home', name: 'home', component: {} },
  { path: '/books', name: 'library', component: {} },
  { path: '/books/:id', name: 'book-detail', component: {} },
  { path: '/books/:bookId/sources', name: 'book-sources-edit', component: {} },
  { path: '/reader/:id', name: 'reader', component: {} },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: {} }
];

function makeRouter(): Router {
  return createRouter({ history: createMemoryHistory(), routes });
}

function nextNavigation(router: Router): Promise<void> {
  return new Promise((resolve) => {
    const remove = router.afterEach(() => {
      remove();
      resolve();
    });
  });
}

describe('safe PlainShelf back navigation', () => {
  it.each([
    '/home',
    '/books?search=ursula&sort=title&folder=scifi&page=3',
    '/books/book-1'
  ])('accepts the PlainShelf source %s', (source) => {
    const router = makeRouter();

    expect(isSafePlainShelfBackTarget(router, '/reader/book-1', source)).toBe(true);
  });

  it.each([
    null,
    'https://example.com/books',
    '//example.com/books',
    '/unknown',
    '/reader/book-1',
    '/books\\book-1'
  ])('rejects an unsafe or unusable predecessor: %s', (source) => {
    const router = makeRouter();

    expect(isSafePlainShelfBackTarget(router, '/reader/book-1', source)).toBe(false);
  });

  it('returns through history with the source query intact and does not loop back into the reader', async () => {
    const router = makeRouter();
    await router.push('/home');
    const source = '/books?search=ursula&sort=title&folder=scifi&page=3';
    await router.push(source);
    await router.push('/reader/book-1');

    let navigated = nextNavigation(router);
    navigateBackSafely(router, router.currentRoute.value.fullPath, '/books/book-1', { back: source });
    await navigated;
    expect(router.currentRoute.value.fullPath).toBe(source);

    navigated = nextNavigation(router);
    router.back();
    await navigated;
    expect(router.currentRoute.value.fullPath).toBe('/home');
  });

  it('replaces a direct deep link with the caller fallback', async () => {
    const router = makeRouter();
    await router.push('/reader/book-1');
    const replace = vi.spyOn(router, 'replace');
    const navigated = nextNavigation(router);

    navigateBackSafely(router, router.currentRoute.value.fullPath, '/books/book-1', { back: null });
    await navigated;

    expect(replace).toHaveBeenCalledWith('/books/book-1');
    expect(router.currentRoute.value.fullPath).toBe('/books/book-1');
  });
});
