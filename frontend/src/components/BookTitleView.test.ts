// @vitest-environment jsdom
import { createApp, defineComponent, h, type App } from 'vue';
import { afterEach, describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import BookTitleView from './BookTitleView.vue';

const mounted: Array<{ app: App; host: HTMLElement }> = [];

function book(overrides: Partial<Book> = {}): Book {
  return { id: 'book-1', title: 'One', authors: [], tags: [], folders: [], ...overrides };
}

function mountTitles(books: Book[]): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(BookTitleView, { books }) }));
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

// The end-to-end case here drove Enter and then Space on a focused row and
// expected the book to open. Both are the browser's own behavior for a button
// and neither is for a div with a click handler, so what actually has to hold
// is that the row is a button — which is also what puts it in the tab order and
// gives a screen reader something to announce.
describe('BookTitleView row activation', () => {
  it('renders each row as a button, so it is focusable and keyboard-operable', () => {
    const row = mountTitles([book({ title: 'Solaris' })]).querySelector('.book-title-row');

    expect(row?.tagName).toBe('BUTTON');
    // type=button: inside a form a submit button would navigate instead.
    expect(row?.getAttribute('type')).toBe('button');
    expect(row?.hasAttribute('disabled')).toBe(false);
  });

  it('reports selection state on the row itself rather than only in a class', () => {
    const host = mountTitles([book({ id: 'book-1' })]);
    const row = host.querySelector('.book-title-row');

    // Not selectable: no pressed state to announce at all.
    expect(row?.getAttribute('aria-pressed')).toBeNull();
  });
});
