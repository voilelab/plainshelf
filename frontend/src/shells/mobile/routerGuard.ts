import type { Router } from 'vue-router';

import {
  MOBILE_BLOCKED_ROUTES,
  stripMobileBlockedQuery
} from '@/features/mobile/utils/blockedRoutes';
import { isShelfEntryUsable, loadShelfEntries } from '@/providers/mobileConfig';

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
