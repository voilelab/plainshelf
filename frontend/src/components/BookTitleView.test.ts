// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import BookTitleView from './BookTitleView.vue';
import { mount } from '#testing/mount';


function book(overrides: Partial<Book> = {}): Book {
  return { id: 'book-1', title: 'One', authors: [], tags: [], folders: [], ...overrides };
}

function mountTitles(books: Book[]): HTMLElement {
  return mount(BookTitleView, { props: { books } }).host;
}

describe('BookTitleView download state', () => {
  it('marks each row with the state it carries', () => {
    const host = mountTitles([
      book({ id: 'a', download_state: 'not_downloaded' }),
      book({ id: 'b', download_state: 'downloaded' }),
      book({ id: 'c', download_state: 'update_available' })
    ]);

    const states = [...host.querySelectorAll('.book-download-badge')]
      .map((badge) => badge.getAttribute('data-download-state'));
    expect(states).toEqual(['not_downloaded', 'downloaded', 'update_available']);
  });

  it('leaves rows unmarked when the backend has no download concept', () => {
    expect(mountTitles([book()]).querySelector('.book-download-badge')).toBeNull();
  });
});

describe('BookTitleView row activation', () => {
  it('renders each row as a button, so it is focusable and keyboard-operable', () => {
    const row = mountTitles([book({ title: 'Solaris' })]).querySelector('.book-title-row');

    expect(row?.tagName).toBe('BUTTON');
    expect(row?.getAttribute('type')).toBe('button');
    expect(row?.hasAttribute('disabled')).toBe(false);
  });

  it('reports selection state on the row itself rather than only in a class', () => {
    const host = mountTitles([book({ id: 'book-1' })]);
    const row = host.querySelector('.book-title-row');

    expect(row?.getAttribute('aria-pressed')).toBeNull();
  });
});
