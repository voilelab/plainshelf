import type { Router } from 'vue-router';

import { READER_BLOCKED_ROUTES, stripReaderBlockedQuery } from './blockedRoutes';

/**
 * The reader shell's route policy.
 *
 * Unlike the mobile shell there is nothing to configure — the binary was pointed
 * at a shelf on the command line, and the server enumerates it — so this only
 * refuses the pages the reading server does not serve.
 *
 * Registered from the shell rather than written into router.ts for the same
 * reason the mobile guard is: the shared router stays free of any one host's
 * policy.
 *
 * Must run before `app.use(router)`, which is what triggers the first
 * navigation. main.ts installs the shell, and then its guards, ahead of that.
 */
export function installReaderRouterGuards(router: Router): void {
  router.beforeEach((to) => {
    if (typeof to.name === 'string' && READER_BLOCKED_ROUTES.has(to.name)) {
      return { name: 'library' };
    }

    // Route names cannot express every write surface: `/import` redirects to
    // `/books?import=1`, and the modal opens off that query alone. Stripping it
    // here keeps the whole route policy in this guard; LibraryPage's own check
    // stays as well, which is deliberate depth rather than duplication.
    const sanitizedQuery = stripReaderBlockedQuery(to.query);
    if (sanitizedQuery) {
      return { path: to.path, query: sanitizedQuery, hash: to.hash, replace: true };
    }

    return true;
  });
}
