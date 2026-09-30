// @vitest-environment jsdom
import { createApp, defineComponent, h, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({ name: 'BookCoverImgStub', setup: () => () => h('div') })
}));

import BookListView from './BookListView.vue';

const mounted: Array<{ app: App; host: HTMLElement }> = [];

function book(overrides: Partial<Book> = {}): Book {
  return { id: 'book-1', title: 'One', authors: [], tags: [], folders: [], ...overrides };
}

function mountList(books: Book[]): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(BookListView, { books }) }));
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

describe('BookListView download state', () => {
  it('marks each row with the state it carries', () => {
    const host = mountList([
      book({ id: 'a', download_state: 'not_downloaded' }),
      book({ id: 'b', download_state: 'downloaded' }),
      book({ id: 'c', download_state: 'update_available' })
    ]);

    const states = [...host.querySelectorAll('.book-download-badge')]
      .map((badge) => badge.getAttribute('data-download-state'));
    expect(states).toEqual(['not_downloaded', 'downloaded', 'update_available']);
  });

  it('leaves rows unmarked when the backend has no download concept', () => {
    expect(mountList([book()]).querySelector('.book-download-badge')).toBeNull();
  });
});
