/**
 * @vitest-environment jsdom
 */
import { ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';
import type { TaskChain, TaskStatus } from '@/types/task';

const mocks = vi.hoisted(() => ({
  copyBook: vi.fn(),
  transferBook: vi.fn(),
  getTaskChain: vi.fn(),
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
  // undefined models a provider without the desktop reader (web/mobile); a
  // spy models the desktop provider.
  openDesktopReader: undefined as undefined | ((bookId: string, section?: number) => Promise<void>),
  layers: ['', 'fiction', 'notes']
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push, resolve: mocks.resolve })
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks,
  getBookshelfProvider: () =>
    mocks.openDesktopReader ? { openDesktopReader: mocks.openDesktopReader } : {},
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
    mocks.openDesktopReader = undefined;
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

  it('opens the standalone reader on the desktop app instead of navigating in place', () => {
    const openReader = vi.fn().mockResolvedValue(undefined);
    mocks.openDesktopReader = openReader;
    const actions = useBookActions();

    actions.goRead('book-1');

    // No section requested: the reader opens at the restored progress.
    expect(openReader).toHaveBeenCalledWith('book-1', undefined);
    expect(mocks.push).not.toHaveBeenCalled();
    expect(openSpy).not.toHaveBeenCalled();
  });

  it('falls back to the in-app reader when the standalone reader will not launch', async () => {
    const openReader = vi.fn().mockRejectedValue(new Error('not installed'));
    mocks.openDesktopReader = openReader;
    const actions = useBookActions();

    actions.goRead('book-1');

    await vi.waitFor(() => expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1' }));
    expect(openReader).toHaveBeenCalledWith('book-1', undefined);
  });

  it('opens the standalone reader at the chapter on a desktop chapter jump', () => {
    const openReader = vi.fn().mockResolvedValue(undefined);
    mocks.openDesktopReader = openReader;
    const actions = useBookActions();

    actions.goRead('book-1', 3);

    // A chapter jump now shells out to the standalone reader too, passing the
    // section so it opens on that chapter rather than being kept in-app.
    expect(openReader).toHaveBeenCalledWith('book-1', 3);
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('falls back to the in-app reader at the chapter when a chapter jump will not launch', async () => {
    const openReader = vi.fn().mockRejectedValue(new Error('not installed'));
    mocks.openDesktopReader = openReader;
    const actions = useBookActions();

    actions.goRead('book-1', 3);

    await vi.waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1', query: { section: '3' } })
    );
    expect(openReader).toHaveBeenCalledWith('book-1', 3);
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

function chain(status: TaskStatus, percentage: number): TaskChain {
  return { id: 'chain-1', name: 'book_transfer', title: 'Transfer', status, percentage, tasks: [] };
}

const flush = (): Promise<void> => Promise.resolve().then(() => undefined);

describe('useBookActions transfer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('forwards the chosen destination and reports completion once the chain settles', async () => {
    mocks.transferBook.mockResolvedValue('chain-1');
    mocks.getTaskChain.mockResolvedValue(chain('completed', 100));
    const onTransferred = vi.fn();

    const actions = useBookActions({ onTransferred });
    actions.requestTransfer(book());
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetLayer: 'notes', mode: 'move' });

    expect(mocks.transferBook).toHaveBeenCalledWith('book-1', 'shelf-b', 'notes', 'move');

    await vi.advanceTimersByTimeAsync(600);
    await flush();

    expect(actions.transferFinished.value).toBe(true);
    expect(actions.transferStatus.value).toBe('completed');
    expect(mocks.fetchBooks).toHaveBeenCalled();
    expect(onTransferred).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'book-1' }),
      'move',
      'completed'
    );
  });

  it('refuses to close while the transfer is still running', async () => {
    mocks.transferBook.mockResolvedValue('chain-1');
    mocks.getTaskChain.mockResolvedValue(chain('running', 40));

    const actions = useBookActions();
    actions.requestTransfer(book());
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetLayer: '', mode: 'copy' });

    expect(actions.transferring.value).toBe(true);
    actions.cancelTransfer();
    expect(actions.transferTarget.value).not.toBeNull();
  });

  it('does not run the post-transfer side effects when the chain fails', async () => {
    mocks.transferBook.mockResolvedValue('chain-1');
    mocks.getTaskChain.mockResolvedValue(chain('failed', 20));
    const onTransferred = vi.fn();

    const actions = useBookActions({ onTransferred });
    actions.requestTransfer(book());
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetLayer: '', mode: 'move' });

    await vi.advanceTimersByTimeAsync(600);
    await flush();

    expect(actions.transferStatus.value).toBe('failed');
    expect(onTransferred).not.toHaveBeenCalled();
  });
});
