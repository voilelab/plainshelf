import type { RouteLocationNormalized, RouteLocationRaw, Router } from 'vue-router';

import {
  MOBILE_BLOCKED_ROUTES,
  stripMobileBlockedQuery
} from '@/features/mobile/utils/blockedRoutes';
import {
  DOWNLOAD_REQUIRED_QUERY_KEY,
  isReadableWhenDownloadRequired
} from '@/features/mobile/utils/readerDownloadGate';
import { getBookshelfProvider } from '@/providers';
import { isShelfEntryUsable, loadShelfEntries } from '@/providers/mobileConfig';
import type { DownloadState } from '@/types/book';

const MOBILE_SETUP_ROUTES = new Set(['mobile-shelves', 'mobile-shelf-add', 'mobile-shelf-edit']);

// Vue Router 5.3.1 installation starts the initial browser navigation; guards must be registered before installation.
export function installMobileRouterGuards(router: Router): void {
  router.beforeEach(async (to) => {
    if (typeof to.name === 'string' && MOBILE_SETUP_ROUTES.has(to.name)) {
      return true;
    }

    const { entries, activeEntryID } = await loadShelfEntries();
    const activeEntry = entries.find((entry) => entry.id === activeEntryID) ?? null;
    if (!(await isShelfEntryUsable(activeEntry))) {
      return { name: 'mobile-shelves', query: to.query };
    }

    if (activeEntry?.type === 'local') {
      return true;
    }

    if (typeof to.name === 'string' && MOBILE_BLOCKED_ROUTES.has(to.name)) {
      return { name: 'library' };
    }

    if (to.name === 'reader') {
      const readerRedirect = await redirectUnlessDownloadedBookIsReadable(to);
      if (readerRedirect) {
        return readerRedirect;
      }
    }

    const sanitizedQuery = stripMobileBlockedQuery(to.query);
    if (sanitizedQuery) {
      return { path: to.path, query: sanitizedQuery, hash: to.hash, replace: true };
    }

    return true;
  });
}

async function redirectUnlessDownloadedBookIsReadable(
  to: RouteLocationNormalized
): Promise<RouteLocationRaw | null> {
  const rawID = to.params.id;
  const bookID = Array.isArray(rawID) ? rawID[0] : rawID;
  if (!bookID) {
    return null;
  }

  const provider = getBookshelfProvider();
  if (!provider.getDownloadState) {
    return null;
  }

  let state: DownloadState;
  try {
    state = await provider.getDownloadState(bookID);
  } catch {
    state = 'not_downloaded';
  }

  if (isReadableWhenDownloadRequired(state)) {
    return null;
  }

  return {
    name: 'book-detail',
    params: { id: bookID },
    query: { ...to.query, [DOWNLOAD_REQUIRED_QUERY_KEY]: '1' }
  };
}
