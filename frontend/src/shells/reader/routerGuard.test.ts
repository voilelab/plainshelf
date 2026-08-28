/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it } from 'vitest';
import {
  createRouter,
  createMemoryHistory,
  isNavigationFailure,
  NavigationFailureType,
  type Router
} from 'vue-router';

import { installReaderRouterGuards, READER_EMPTY_ROUTE } from './routerGuard';

const BOOK_ID = 'dune1234';

function stubBootConfig(bookID: string | null, section?: number): void {
  if (bookID === null) {
    delete (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__;
    return;
  }
  (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__ = {
    shelf_id: 'book',
    book_id: bookID,
    ...(section === undefined ? {} : { section })
  };
}

// A stand-in for the shared router: the same route names the app registers,
// with placeholder components, so the guard is exercised against real
// navigation rather than a mock.
function newRouter(): Router {
  const blank = { template: '<div />' };
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', redirect: '/books' },
      { path: '/books', name: 'library', component: blank },
      { path: '/books/:id', name: 'book-detail', component: blank },
      { path: '/settings', name: 'settings', component: blank },
      { path: '/reader/:id', name: 'reader', component: blank },
      { path: '/books/:bookId/sources', name: 'book-sources-edit', component: blank }
    ]
  });
  installReaderRouterGuards(router);
  return router;
}

describe('reader router guards', () => {
  beforeEach(() => {
    stubBootConfig(BOOK_ID);
  });

  it('sends every route to the open book', async () => {
    const router = newRouter();

    for (const path of ['/', '/books', '/books/other', '/settings', '/books/other/sources']) {
      await router.push(path);
      expect(router.currentRoute.value.name).toBe('reader');
      expect(router.currentRoute.value.params.id).toBe(BOOK_ID);
    }
  });

  it('stays on the reader once it is showing the open book', async () => {
    const router = newRouter();

    await router.push(`/reader/${BOOK_ID}`);
    expect(router.currentRoute.value.name).toBe('reader');
    expect(router.currentRoute.value.params.id).toBe(BOOK_ID);
  });

  it('cancels attempts to leave the open book instead of redirecting back into it', async () => {
    const router = newRouter();
    await router.push(`/reader/${BOOK_ID}?section=3`);

    const result = await router.push('/books');

    expect(isNavigationFailure(result, NavigationFailureType.aborted)).toBe(true);
    expect(router.currentRoute.value.fullPath).toBe(`/reader/${BOOK_ID}?section=3`);
  });

  // The reader serves exactly one book, so a link to another id would 404
  // against its own API rather than open anything.
  it('redirects a reader link for another book to the open one', async () => {
    const router = newRouter();

    await router.push('/reader/someoneelse');
    expect(router.currentRoute.value.params.id).toBe(BOOK_ID);
  });

  // A reader launched for a chapter carries the section as the `?section=` deep
  // link, so the standalone window opens on that chapter rather than the restored
  // progress. Section 0 (the first chapter) is a real request and must be carried.
  it('carries the launch section into the reader route', async () => {
    for (const section of [3, 0]) {
      stubBootConfig(BOOK_ID, section);
      const router = newRouter();

      await router.push('/books');
      expect(router.currentRoute.value.name).toBe('reader');
      expect(router.currentRoute.value.params.id).toBe(BOOK_ID);
      expect(router.currentRoute.value.query.section).toBe(String(section));
    }
  });

  // Without a launch section the redirect stays query-free, so the reader opens
  // at the restored progress just as a default read does.
  it('omits the section when none was requested', async () => {
    const router = newRouter();

    await router.push('/books');
    expect(router.currentRoute.value.name).toBe('reader');
    expect(router.currentRoute.value.query.section).toBeUndefined();
  });

  it('holds on an empty route until a book is opened', async () => {
    stubBootConfig('');
    const router = newRouter();

    await router.push('/books');
    expect(router.currentRoute.value.name).toBe(READER_EMPTY_ROUTE);
  });

  it('sends the holding route to the book once one is open', async () => {
    stubBootConfig('');
    const router = newRouter();
    await router.push('/books');

    stubBootConfig(BOOK_ID);
    await router.push('/books');
    expect(router.currentRoute.value.name).toBe('reader');
    expect(router.currentRoute.value.params.id).toBe(BOOK_ID);
  });
});
