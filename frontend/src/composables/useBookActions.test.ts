/**
 * @vitest-environment jsdom
 */
import { ref } from 'vue';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

const mocks = vi.hoisted(() => ({
  copyBook: vi.fn(),
  push: vi.fn(),
  resolve: vi.fn((to: unknown) => ({
    href:
      typeof to === 'string'
        ? to
        : `${(to as { path: string }).path}${
            (to as { query?: { section?: string } }).query?.section !== undefined
              ? `?section=${(to as { query: { section: string } }).query.section}`
              : ''
          }`
  })),
  fetchBooks: vi.fn(),
  fetchLayers: vi.fn(),
  isWebRuntime: vi.fn(() => false),
  layers: ['', 'fiction', 'notes']
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push, resolve: mocks.resolve })
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks,
  getBookshelfProvider: () => ({}),
  isWebRuntime: mocks.isWebRuntime
}));

vi.mock('@/composables/useLayerStore', () => ({
  useLayerStore: () => ({
    layers: ref(mocks.layers),
    loaded: ref(true),
    fetchLayers: mocks.fetchLayers
  })
}));

vi.mock('@/composables/useBookStore', () => ({
  useBookStore: () => ({ fetchBooks: mocks.fetchBooks })
}));

vi.mock('@/i18n', () => ({ t: (key: string) => key }));

import { useBookActions } from './useBookActions';

function book(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-1',
    title: 'Original',
    authors: ['Ada'],
    layers: ['fiction'],
    language: 'en',
    format: 'txt',
    tags: [],
    star: 0,
    ...overrides
  } as Book;
}

describe('useBookActions goRead', () => {
  let openSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.isWebRuntime.mockReturnValue(false);
    // window.open is unimplemented in jsdom; a spy both silences it and lets us
    // assert the new-tab call. Default to a truthy handle (pop-up allowed).
    openSpy = vi.spyOn(window, 'open').mockReturnValue({} as Window);
  });

  it('opens the reader in a new tab on a web build instead of navigating in place', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    const actions = useBookActions();

    actions.goRead('book-1');

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('carries the section index into the new-tab URL on a web build', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    const actions = useBookActions();

    actions.goRead('book-1', 3);

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1?section=3', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('navigates in place via router.push on a non-web build', () => {
    mocks.isWebRuntime.mockReturnValue(false);
    const actions = useBookActions();

    actions.goRead('book-1', 3);

    expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1', query: { section: '3' } });
    expect(openSpy).not.toHaveBeenCalled();
  });

  // noopener/noreferrer make window.open return null even on success, so the web
  // path must never touch router.push — doing so would navigate the original tab
  // on every successful open, defeating the new-tab behaviour. openSpy returns
  // null here to model that spec behaviour.
  it('does not navigate the original tab on a web build even when window.open returns null', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    openSpy.mockReturnValue(null);
    const actions = useBookActions();

    actions.goRead('book-1');

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });
});

describe('useBookActions copy', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('copies the book and hands the fresh copy to onCopied', async () => {
    const copy = book({ id: 'book-2', layers: ['notes'] });
    mocks.copyBook.mockResolvedValue(copy);
    const onCopied = vi.fn();

    const actions = useBookActions({ onCopied });
    actions.requestCopy(book());
    await actions.submitCopy('notes');

    expect(mocks.copyBook).toHaveBeenCalledWith('book-1', 'notes');
    expect(onCopied).toHaveBeenCalledWith(copy, 'notes');
    expect(mocks.fetchBooks).toHaveBeenCalled();
    expect(actions.copyTarget.value).toBeNull();
    expect(actions.actionError.value).toBe('');
  });

  // Copying into the book's own layer is a real "duplicate here", not a no-op,
  // so the current layer must stay in the offered destinations.
  it('keeps the book\'s own layer among the copy destinations', () => {
    const actions = useBookActions();
    actions.requestCopy(book({ layers: ['fiction'] }));

    expect(actions.copyLayerOptions.value).toContain('fiction');
  });

  it('keeps the dialog open with an error when the copy fails', async () => {
    mocks.copyBook.mockRejectedValue(new Error('disk full'));
    const onCopied = vi.fn();

    const actions = useBookActions({ onCopied });
    actions.requestCopy(book());
    await actions.submitCopy('notes');

    expect(onCopied).not.toHaveBeenCalled();
    expect(actions.copyTarget.value).not.toBeNull();
    expect(actions.actionError.value).toBe('disk full');
    expect(actions.copying.value).toBe(false);
  });
});
