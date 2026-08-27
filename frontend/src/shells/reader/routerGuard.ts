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

  router.beforeEach((to, from) => {
    const boot = readerBootConfig();
    const bookID = boot?.book_id ?? '';

    if (!bookID) {
      return to.name === READER_EMPTY_ROUTE ? true : { name: READER_EMPTY_ROUTE };
    }

    // Already reading the book this app was opened with. Compared rather than
    // trusted: the reader serves exactly one book, and a link to any other id
    // would 404 against its own API. The launch redirect below sets the section
    // once; leaving an in-place navigation untouched here is what stops a later
    // chapter change from being dragged back to the launch chapter.
    if (to.name === 'reader' && to.params.id === bookID) {
      return true;
    }

    // Once the standalone shell is reading its one book, it has nowhere else
    // inside PlainShelf to go. Cancel browser/mouse back and stray links in
    // place instead of redirecting them to the same Reader route: redirecting
    // re-entered ReaderView and could reload the book or repeat its launch
    // chapter. Closing the native window is the shell's exit action.
    if (from.name === 'reader' && from.params.id === bookID) {
      return false;
    }

    // A reader launched for a specific chapter (desktop -section) carries it as
    // the `?section=` deep link ReaderView honours on load, so the standalone
    // window opens on that chapter rather than the restored progress.
    const section = boot?.section ?? null;
    return section === null
      ? { name: 'reader', params: { id: bookID } }
      : { name: 'reader', params: { id: bookID }, query: { section: String(section) } };
  });
}
