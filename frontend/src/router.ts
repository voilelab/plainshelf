import { createRouter, createWebHistory } from 'vue-router';

declare module 'vue-router' {
  interface RouteMeta {
    shelfOptional?: boolean;
    backTo?: { path: string; query?: Record<string, string> };
  }
}
import MainLayout from '@/layouts/MainLayout.vue';
import ReaderLayout from '@/layouts/ReaderLayout.vue';
import { APP_TITLE } from '@/composables/useDocumentTitle';

const DashboardPage = () => import('@/features/dashboard/pages/DashboardPage.vue');
const LibraryPage = () => import('@/features/library/pages/LibraryPage.vue');
const BookDetailPage = () => import('@/features/library/pages/BookDetailPage.vue');
const DuplicateContentPage = () => import('@/features/maintenance/pages/DuplicateContentPage.vue');
const SimilarContentPage = () => import('@/features/maintenance/pages/SimilarContentPage.vue');
const ReadHistoryPage = () => import('@/pages/ReadHistoryPage.vue');
const SeriesListPage = () => import('@/features/library/pages/SeriesListPage.vue');
const SeriesDetailPage = () => import('@/features/library/pages/SeriesDetailPage.vue');
const TrashPage = () => import('@/features/trash/pages/TrashPage.vue');
const DownloadsPage = () => import('@/features/mobile/pages/DownloadsPage.vue');
const AdminLogsPage = () => import('@/features/settings/pages/AdminLogsPage.vue');
const SettingsPage = () => import('@/features/settings/pages/SettingsPage.vue');
const MobileShelfEditPage = () => import('@/features/mobile/pages/MobileShelfEditPage.vue');
const ReaderPage = () => import('@/features/reader/pages/ReaderView.vue');
const EditBookSourcesPage = () => import('@/features/sources/pages/EditBookSourcesPage.vue');
const NotFoundPage = () => import('@/pages/NotFoundPage.vue');

const ROUTES_WITH_OWN_TITLE = new Set([
  'home',
  'library',
  'book-detail',
  'book-sources-edit',
  'reader',
  'read-history',
  'series-list',
  'series-detail',
  'trash',
  'downloads',
  'admin-logs',
  'settings',
  'similar-content',
  'not-found'
]);

const MANAGE_SHELVES_BACK = { path: '/settings', query: { tab: 'shelves' } };

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/home'
    },
    {
      path: '/',
      component: MainLayout,
      children: [
        {
          path: 'home',
          name: 'home',
          component: DashboardPage
        },
        {
          path: 'dashboard',
          redirect: (to) => ({ path: '/home', query: { ...to.query } })
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
          path: 'series',
          name: 'series-list',
          component: SeriesListPage
        },
        {
          path: 'series/:name',
          name: 'series-detail',
          component: SeriesDetailPage,
          props: true
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
          redirect: (to) => ({ path: '/books', query: { ...to.query, author: 'none' } })
        },
        {
          path: 'books/maintenance/missing-cover',
          redirect: (to) => ({ path: '/books', query: { ...to.query, cover: 'none' } })
        },
        {
          path: 'books/maintenance/missing-language',
          redirect: (to) => ({ path: '/books', query: { ...to.query, language: 'none' } })
        },
        {
          path: 'admin/logs',
          name: 'admin-logs',
          component: AdminLogsPage
        },
        {
          path: 'settings',
          name: 'settings',
          component: SettingsPage,
          meta: { shelfOptional: true }
        },
        {
          path: 'settings/shelves/new',
          name: 'mobile-shelf-add',
          component: MobileShelfEditPage,
          meta: { shelfOptional: true, backTo: MANAGE_SHELVES_BACK }
        },
        {
          path: 'settings/shelves/:entryId',
          name: 'mobile-shelf-edit',
          component: MobileShelfEditPage,
          meta: { shelfOptional: true, backTo: MANAGE_SHELVES_BACK }
        },
        {
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
