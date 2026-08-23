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

// The shelf-list routes: reachable even with nothing configured, because they
// are where the user configures it.
const MOBILE_SETUP_ROUTES = new Set(['mobile-shelves', 'mobile-shelf-add', 'mobile-shelf-edit']);

/**
 * The mobile shell's route policy.
 *
 * On the native shell there is no backend to inject a server address or a
 * selected shelf, so every route is gated behind a usable shelf entry, and the
 * pages a read-only client cannot use are refused outright.
 *
 * Registered from the shell rather than written into router.ts so the shared
 * router does not import mobileConfig — which is what kept
 * `@capacitor/preferences` and the Keystore wrapper in the web and desktop
 * bundles. The policy itself is unchanged; only who installs it moved.
 *
 * Must run before `app.use(router)`, which is what triggers the first
 * navigation. main.ts installs the shell, and then its guards, ahead of that.
 */
export function installMobileRouterGuards(router: Router): void {
  router.beforeEach(async (to) => {
    if (typeof to.name === 'string' && MOBILE_SETUP_ROUTES.has(to.name)) {
      return true;
    }

    const { entries, activeEntryID } = await loadShelfEntries();
    const activeEntry = entries.find((entry) => entry.id === activeEntryID) ?? null;
    if (!(await isShelfEntryUsable(activeEntry))) {
      // Carries the query so a redirect cannot drop `?mobile-shell-preview=1`,
      // which is what keeps the browser preview in mobile mode across the
      // reload that saving a shelf performs.
      return { name: 'mobile-shelves', query: to.query };
    }

    if (typeof to.name === 'string' && MOBILE_BLOCKED_ROUTES.has(to.name)) {
      return { name: 'library' };
    }

    if (to.name === 'reader') {
      const readerRedirect = await readerDownloadRedirect(to);
      if (readerRedirect) {
        return readerRedirect;
      }
    }

    // Route names cannot express every write surface: `/import` redirects to
    // `/books?import=1`, and the modal opens off that query alone. Strip it here
    // so the whole mobile route policy lives in this guard. LibraryPage keeps its
    // own check as well — that is deliberate depth, not duplication.
    const sanitizedQuery = stripMobileBlockedQuery(to.query);
    if (sanitizedQuery) {
      return { path: to.path, query: sanitizedQuery, hash: to.hash, replace: true };
    }

    return true;
  });
}

/**
 * Enforces "download before read" for the reader route: resolves the target
 * book's on-device download state and, unless a readable copy exists, redirects
 * to the book's detail page with a flag that page shows a download prompt for.
 *
 * Returns null to let the navigation proceed — when there is no book id, when
 * the active provider has no download concept (so the gate cannot apply), or
 * when the book is already downloaded. A failure reading the state is treated
 * as "not downloaded" so the requirement holds rather than leaking through on a
 * transient cache error; the detail page it lands on can retry the download.
 */
async function readerDownloadRedirect(
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
