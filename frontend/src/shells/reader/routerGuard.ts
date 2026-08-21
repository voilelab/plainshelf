import type { Router } from 'vue-router';

import { readerBootConfig } from './bootConfig';
import ReaderEmptyView from './ReaderEmptyView.vue';

/**
 * Where the app waits when the reader has no book yet — the user cancelled the
 * folder dialog, and File → Open Book is the way back in.
 *
 * Reached only after the user cancels: on first launch the native dialog is
 * already in front of the window, and this is what is behind it.
 */
export const READER_EMPTY_ROUTE = 'reader-app-empty';

/**
 * The reader app's route policy: there is one book, so there is one page.
 *
 * Registered from the shell rather than written into router.ts for the same
 * reason the mobile guards are — the shared router stays free of any single
 * host's rules — and installed before `app.use(router)`, which is what triggers
 * the first navigation.
 */
export function installReaderRouterGuards(router: Router): void {
  router.addRoute({
    path: '/reader-app',
    name: READER_EMPTY_ROUTE,
    component: ReaderEmptyView
  });

  router.beforeEach((to) => {
    const bookID = readerBootConfig()?.book_id ?? '';

    if (!bookID) {
      return to.name === READER_EMPTY_ROUTE ? true : { name: READER_EMPTY_ROUTE };
    }

    // Already reading the book this app was opened with. Compared rather than
    // trusted: the reader serves exactly one book, and a link to any other id
    // would 404 against its own API.
    if (to.name === 'reader' && to.params.id === bookID) {
      return true;
    }

    return { name: 'reader', params: { id: bookID } };
  });
}
