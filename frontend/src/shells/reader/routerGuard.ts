import type { Router } from 'vue-router';

import { readerBootConfig } from './bootConfig';
import ReaderEmptyView from './ReaderEmptyView.vue';

export const READER_EMPTY_ROUTE = 'reader-app-empty';

// Vue Router 5.3.1 installation starts the initial browser navigation; guards must be registered before installation.
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

    if (to.name === 'reader' && to.params.id === bookID) {
      return true;
    }

    if (from.name === 'reader' && from.params.id === bookID) {
      return false;
    }

    const section = boot?.section ?? null;
    return section === null
      ? { name: 'reader', params: { id: bookID } }
      : { name: 'reader', params: { id: bookID }, query: { section: String(section) } };
  });
}
