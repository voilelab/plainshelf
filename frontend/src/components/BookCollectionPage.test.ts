// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import { afterEach, describe, expect, it } from 'vitest';
import BookCollectionPage from './BookCollectionPage.vue';
import type { Book } from '@/types/book';


function book(id: string, title: string): Book {
  return { id, title } as Book;
}

let active: { app: App; host: HTMLElement } | null = null;

async function mountCollection(props: {
  books: Book[];
  total: number;
  emptyMessage?: string;
}): Promise<HTMLElement> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/books', component: defineComponent({ render: () => null }) }]
  });
  await router.push('/books');
  await router.isReady();

  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(
    defineComponent({
      setup: () => () =>
        h(BookCollectionPage, {
          title: 'All books',
          page: 1,
          pageSize: 10,
          emptyMessage: props.emptyMessage ?? 'No books found.',
          ...props
        })
    })
  );
  app.use(router);
  app.mount(host);
  active = { app, host };
  await nextTick();
  return host;
}

afterEach(() => {
  active?.app.unmount();
  active?.host.remove();
  active = null;
});

describe('BookCollectionPage pagination row', () => {
  it('shows the empty state with no pager when nothing matched', async () => {
    const host = await mountCollection({
      books: [],
      total: 0,
      emptyMessage: 'No books found for "nothing-matches-this".'
    });

    expect(host.querySelector('.empty-state')?.textContent).toContain(
      'No books found for "nothing-matches-this".'
    );
    expect(host.querySelector('.pagination')).toBeNull();
  });

  it('shows the pager once there is something to page through', async () => {
    const host = await mountCollection({ books: [book('b1', 'Solaris')], total: 1 });

    expect(host.querySelector('.empty-state')).toBeNull();
    expect(host.querySelector('.pagination')).not.toBeNull();
  });
});
