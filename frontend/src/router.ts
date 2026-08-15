import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from '@/layouts/MainLayout.vue';
import ReaderLayout from '@/layouts/ReaderLayout.vue';
import { APP_TITLE } from '@/composables/useDocumentTitle';
import { isMobileRuntime } from '@/providers/runtime';
import { isShelfEntryUsable, loadShelfEntries } from '@/providers/mobileConfig';
import {
  MOBILE_BLOCKED_ROUTES,
  stripMobileBlockedQuery
} from '@/features/mobile/utils/blockedRoutes';

const DashboardPage = () => import('@/features/dashboard/pages/DashboardPage.vue');
const LibraryPage = () => import('@/features/library/pages/LibraryPage.vue');
const BookDetailPage = () => import('@/features/library/pages/BookDetailPage.vue');
const EditBookPage = () => import('@/features/library/pages/EditBookPage.vue');
const DuplicateContentPage = () => import('@/features/maintenance/pages/DuplicateContentPage.vue');
const MissingAuthorPage = () => import('@/features/maintenance/pages/MissingAuthorPage.vue');
const MissingCoverPage = () => import('@/features/maintenance/pages/MissingCoverPage.vue');
const MissingLanguagePage = () => import('@/features/maintenance/pages/MissingLanguagePage.vue');
const ReadHistoryPage = () => import('@/pages/ReadHistoryPage.vue');
const TrashPage = () => import('@/features/trash/pages/TrashPage.vue');
const DownloadsPage = () => import('@/features/mobile/pages/DownloadsPage.vue');
const AdminLogsPage = () => import('@/features/settings/pages/AdminLogsPage.vue');
const SettingsPage = () => import('@/features/settings/pages/SettingsPage.vue');
const MobileShelvesPage = () => import('@/features/mobile/pages/MobileShelvesPage.vue');
const MobileConnectPage = () => import('@/features/mobile/pages/MobileConnectPage.vue');
const ReaderPage = () => import('@/features/reader/pages/ReaderView.vue');
const EditBookSourcesPage = () => import('@/features/sources/pages/EditBookSourcesPage.vue');

const ROUTES_WITH_OWN_TITLE = new Set([
  'dashboard',
  'library',
  'book-detail',
  'book-sources-edit',
  'reader',
  'read-history',
  'trash',
  'downloads',
  'admin-logs',
  'settings',
  'maintenance-missing-author',
  'maintenance-missing-cover',
  'maintenance-missing-language'
]);

// A route that edits the shelf, administers the server, or opens a write
// surface from a query parameter also belongs in
// features/mobile/utils/blockedRoutes.ts — the mobile shell is a read-only
// reading client and the guard below is what keeps it that way.
const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard'
    },
    {
      path: '/connect',
      name: 'mobile-shelves',
      component: MobileShelvesPage
    },
    {
      path: '/connect/new',
      name: 'mobile-shelf-add',
      component: MobileConnectPage
    },
    {
      path: '/connect/:entryId',
      name: 'mobile-shelf-edit',
      component: MobileConnectPage
    },
    {
      path: '/',
      component: MainLayout,
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: DashboardPage
        },
        {
          path: 'books',
          name: 'library',
          component: LibraryPage
        },
        {
          path: 'books/:id',
          name: 'book-detail',
          component: BookDetailPage,
          props: true
        },
        {
          path: 'books/:id/edit',
          name: 'book-edit',
          component: EditBookPage,
          props: true
        },
        {
          path: 'import',
          name: 'import',
          redirect: (to) => ({
            path: '/books',
            query: {
              ...to.query,
              import: '1'
            }
          })
        },
        {
          path: 'duplicates',
          name: 'duplicate-content',
          component: DuplicateContentPage
        },
        {
          path: 'read-history',
          name: 'read-history',
          component: ReadHistoryPage
        },
        {
          path: 'trash',
          name: 'trash',
          component: TrashPage
        },
        {
          path: 'downloads',
          name: 'downloads',
          component: DownloadsPage
        },
        {
          path: 'books/maintenance/missing-author',
          name: 'maintenance-missing-author',
          component: MissingAuthorPage
        },
        {
          path: 'books/maintenance/missing-cover',
          name: 'maintenance-missing-cover',
          component: MissingCoverPage
        },
        {
          path: 'books/maintenance/missing-language',
          name: 'maintenance-missing-language',
          component: MissingLanguagePage
        },
        {
          path: 'admin/logs',
          name: 'admin-logs',
          component: AdminLogsPage
        },
        {
          path: 'settings',
          name: 'settings',
          component: SettingsPage
        }
      ]
    },
    {
      path: '/reader/:id',
      component: ReaderLayout,
      children: [
        {
          path: '',
          name: 'reader',
          component: ReaderPage,
          props: true
        }
      ]
    },
    {
      path: '/books/:bookId/sources',
      component: ReaderLayout,
      children: [
        {
          path: '',
          name: 'book-sources-edit',
          component: EditBookSourcesPage,
          props: true
        }
      ]
    },
  ]
});

// The shelf-list routes: reachable even with nothing configured, because they
// are where the user configures it.
const MOBILE_SETUP_ROUTES = new Set(['mobile-shelves', 'mobile-shelf-add', 'mobile-shelf-edit']);

// On the native mobile shell there is no backend to inject a server address or
// selected shelf, so gate every route behind a usable shelf entry.
router.beforeEach(async (to) => {
  if (!isMobileRuntime()) {
    return true;
  }
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

router.afterEach((to) => {
  if (typeof to.name === 'string' && ROUTES_WITH_OWN_TITLE.has(to.name)) {
    return;
  }

  document.title = APP_TITLE;
});

export default router;
