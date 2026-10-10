// @vitest-environment jsdom
import { defineComponent, h } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({ name: 'BookCoverImgStub', setup: () => () => h('div') })
}));

import BookListView from './BookListView.vue';
import { mount } from '#testing/mount';


function book(overrides: Partial<Book> = {}): Book {
  return { id: 'book-1', title: 'One', authors: [], tags: [], folders: [], ...overrides };
}

function mountList(books: Book[]): HTMLElement {
  return mount(BookListView, { props: { books } }).host;
}

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
