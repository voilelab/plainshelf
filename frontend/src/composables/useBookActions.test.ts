import { ref } from 'vue';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

const mocks = vi.hoisted(() => ({
  copyBook: vi.fn(),
  push: vi.fn(),
  fetchBooks: vi.fn(),
  fetchLayers: vi.fn(),
  layers: ['', 'fiction', 'notes']
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push })
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks,
  getBookshelfProvider: () => ({})
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
