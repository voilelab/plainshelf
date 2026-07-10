import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from './layouts/MainLayout.vue';
import ReaderLayout from './layouts/ReaderLayout.vue';
import { APP_TITLE } from './composables/useDocumentTitle';
import { isMobileRuntime } from './providers/runtime';
import { loadMobileConnectionConfig } from './providers/mobileConfig';

const LibraryPage = () => import('./pages/LibraryPage.vue');
const BookDetailPage = () => import('./pages/BookDetailPage.vue');
const EditBookPage = () => import('./pages/EditBookPage.vue');
const DuplicateContentPage = () => import('./pages/DuplicateContentPage.vue');
const MissingAuthorPage = () => import('./pages/MissingAuthorPage.vue');
const MissingCoverPage = () => import('./pages/MissingCoverPage.vue');
const MissingLanguagePage = () => import('./pages/MissingLanguagePage.vue');
const ReadHistoryPage = () => import('./pages/ReadHistoryPage.vue');
const TrashPage = () => import('./pages/TrashPage.vue');
const AdminLogsPage = () => import('./pages/AdminLogsPage.vue');
const SettingsPage = () => import('./pages/SettingsPage.vue');
const MobileConnectPage = () => import('./pages/MobileConnectPage.vue');
const ReaderPage = () => import('./features/reader/views/ReaderView.vue');
const EditBookSourcesPage = () => import('./features/sources/pages/EditBookSourcesPage.vue');

const ROUTES_WITH_OWN_TITLE = new Set([
  'library',
  'book-detail',
  'book-sources-edit',
  'reader',
  'read-history',
  'trash',
  'admin-logs',
  'settings',
  'maintenance-missing-author',
  'maintenance-missing-cover'
]);

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/books'
    },
    {
      path: '/connect',
      name: 'mobile-connect',
      component: MobileConnectPage
    },
    {
      path: '/',
      component: MainLayout,
      children: [
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

// On the native mobile shell there is no backend to inject a server address or
// selected shelf, so gate every route behind a completed connection setup.
router.beforeEach(async (to) => {
  if (!isMobileRuntime()) {
    return true;
  }
  if (to.name === 'mobile-connect') {
    return true;
  }

  const { serverUrl, shelfId } = await loadMobileConnectionConfig();
  // Token is intentionally not required: reads work without it, only writes need one.
  if (!serverUrl || !shelfId) {
    return { name: 'mobile-connect' };
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
