import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from '@/layouts/MainLayout.vue';
import ReaderLayout from '@/layouts/ReaderLayout.vue';
import { APP_TITLE } from '@/composables/useDocumentTitle';
import { isMobileRuntime } from '@/providers/runtime';
import { isConnectionConfigured, loadMobileConnectionConfig } from '@/providers/mobileConfig';
import { MOBILE_BLOCKED_ROUTES } from '@/features/mobile/utils/blockedRoutes';

const DashboardPage = () => import('@/features/dashboard/pages/DashboardPage.vue');
const LibraryPage = () => import('@/features/library/pages/LibraryPage.vue');
const BookDetailPage = () => import('@/features/library/pages/BookDetailPage.vue');
const EditBookPage = () => import('@/features/library/pages/EditBookPage.vue');
const DuplicateContentPage = () => import('@/features/maintenance/pages/DuplicateContentPage.vue');
const MissingAuthorPage = () => import('@/features/maintenance/pages/MissingAuthorPage.vue');
const MissingCoverPage = () => import('@/features/maintenance/pages/MissingCoverPage.vue');
const MissingLanguagePage = () => import('@/features/maintenance/pages/MissingLanguagePage.vue');
const LowCharCountPage = () => import('@/features/maintenance/pages/LowCharCountPage.vue');
const ReadHistoryPage = () => import('@/pages/ReadHistoryPage.vue');
const TrashPage = () => import('@/features/trash/pages/TrashPage.vue');
const DownloadsPage = () => import('@/features/mobile/pages/DownloadsPage.vue');
const AdminLogsPage = () => import('@/features/settings/pages/AdminLogsPage.vue');
const SettingsPage = () => import('@/features/settings/pages/SettingsPage.vue');
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
  'maintenance-missing-language',
  'maintenance-low-char-count'
]);

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard'
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
          path: 'books/maintenance/low-char-count',
          name: 'maintenance-low-char-count',
          component: LowCharCountPage
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

  if (!isConnectionConfigured(await loadMobileConnectionConfig())) {
    return { name: 'mobile-connect' };
  }

  if (typeof to.name === 'string' && MOBILE_BLOCKED_ROUTES.has(to.name)) {
    return { name: 'library' };
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
