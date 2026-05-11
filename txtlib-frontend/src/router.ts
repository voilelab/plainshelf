import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from './layouts/MainLayout.vue';
import ReaderLayout from './layouts/ReaderLayout.vue';
import LibraryPage from './pages/LibraryPage.vue';
import BookDetailPage from './pages/BookDetailPage.vue';
import EditBookPage from './pages/EditBookPage.vue';
import DuplicateContentPage from './pages/DuplicateContentPage.vue';
import MissingAuthorPage from './pages/MissingAuthorPage.vue';
import MissingCoverPage from './pages/MissingCoverPage.vue';
import MissingLanguagePage from './pages/MissingLanguagePage.vue';
import ReaderPage from './pages/ReaderPage.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/books'
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
  ]
});

export default router;
