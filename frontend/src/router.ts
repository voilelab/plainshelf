import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from '@/layouts/MainLayout.vue';
import ReaderLayout from '@/layouts/ReaderLayout.vue';
import { APP_TITLE } from '@/composables/useDocumentTitle';

const DashboardPage = () => import('@/features/dashboard/pages/DashboardPage.vue');
const LibraryPage = () => import('@/features/library/pages/LibraryPage.vue');
const BookDetailPage = () => import('@/features/library/pages/BookDetailPage.vue');
const EditBookPage = () => import('@/features/library/pages/EditBookPage.vue');
const DuplicateContentPage = () => import('@/features/maintenance/pages/DuplicateContentPage.vue');
const SimilarContentPage = () => import('@/features/maintenance/pages/SimilarContentPage.vue');
const ReadHistoryPage = () => import('@/pages/ReadHistoryPage.vue');
const TrashPage = () => import('@/features/trash/pages/TrashPage.vue');
const DownloadsPage = () => import('@/features/mobile/pages/DownloadsPage.vue');
const AdminLogsPage = () => import('@/features/settings/pages/AdminLogsPage.vue');
const SettingsPage = () => import('@/features/settings/pages/SettingsPage.vue');
const MobileShelvesPage = () => import('@/features/mobile/pages/MobileShelvesPage.vue');
const MobileConnectPage = () => import('@/features/mobile/pages/MobileConnectPage.vue');
const ReaderPage = () => import('@/features/reader/pages/ReaderView.vue');
const EditBookSourcesPage = () => import('@/features/sources/pages/EditBookSourcesPage.vue');
const NotFoundPage = () => import('@/pages/NotFoundPage.vue');

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
  'similar-content',
  'not-found'
]);

// A route that edits the shelf, administers the server, or opens a write
// surface from a query parameter also belongs in
// features/mobile/utils/blockedRoutes.ts — the mobile shell is a read-only
// reading client, and the guard that shell installs (shells/mobile/routerGuard)
// is what keeps it that way.
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
          path: 'similar',
          name: 'similar-content',
          component: SimilarContentPage
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
        // The dedicated "missing X" pages are gone: each condition is now a
        // book-list filter, so these paths redirect to the library query that
        // expresses them. Old links and bookmarks keep working.
        {
          path: 'books/maintenance/missing-author',
          redirect: { path: '/books', query: { author: 'none' } }
        },
        {
          path: 'books/maintenance/missing-cover',
          redirect: { path: '/books', query: { cover: 'none' } }
        },
        {
          path: 'books/maintenance/missing-language',
          redirect: { path: '/books', query: { language: 'none' } }
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
        },
        {
          // Lowest-ranked pattern in the table, so it only matches what nothing
          // else did. Without it an unknown URL renders the shell around an
          // empty <RouterView>, which reads as a broken app rather than a bad
          // address. It lives under MainLayout so the sidebar stays usable, and
          // the mobile shell's shelf gate still applies to it: an unconfigured
          // mobile shell is sent to /connect instead.
          path: ':pathMatch(.*)*',
          name: 'not-found',
          component: NotFoundPage
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

router.afterEach((to) => {
  if (typeof to.name === 'string' && ROUTES_WITH_OWN_TITLE.has(to.name)) {
    return;
  }

  document.title = APP_TITLE;
});

export default router;
