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
  fetchBooks: vi.fn(),
  fetchFolders: vi.fn(),
  folders: ['', 'fiction', 'notes']
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push })
}));

// The reader-launch path moved to useReaderLaunch (covered by its own suite);
// stub it so instantiating useBookActions here neither reaches the real router
// nor opens a reader.
vi.mock('@/composables/useReaderLaunch', () => ({
  useReaderLaunch: () => ({ launchReader: vi.fn(), onReaderLinkClick: vi.fn() })
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks,
  getBookshelfProvider: () => ({})
}));

vi.mock('@/composables/useFolderStore', () => ({
  useFolderStore: () => ({
    folders: ref(mocks.folders),
    loaded: ref(true),
    fetchFolders: mocks.fetchFolders
  })
}));

vi.mock('@/composables/useBookStore', () => ({
  useBookStore: () => ({ fetchBooks: mocks.fetchBooks })
}));

vi.mock('@/composables/useToasts', () => ({
  useToasts: () => ({ showToast: vi.fn() })
}));

vi.mock('@/i18n', () => ({ t: (key: string) => key }));

import { useBookActions } from './useBookActions';

function book(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-1',
    title: 'Original',
    authors: ['Ada'],
    folders: ['fiction'],
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
    const copy = book({ id: 'book-2', folders: ['notes'] });
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

  // Copying into the book's own folder is a real "duplicate here", not a no-op,
  // so the current folder must stay in the offered destinations.
  it('keeps the book\'s own folder among the copy destinations', () => {
    const actions = useBookActions();
    actions.requestCopy(book({ folders: ['fiction'] }));

    expect(actions.copyFolderOptions.value).toContain('fiction');
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
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetFolder: 'notes', mode: 'move' });

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
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetFolder: '', mode: 'copy' });

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
    await actions.submitTransfer({ targetShelfId: 'shelf-b', targetFolder: '', mode: 'move' });

    await vi.advanceTimersByTimeAsync(600);
    await flush();

    expect(actions.transferStatus.value).toBe('failed');
    expect(onTransferred).not.toHaveBeenCalled();
  });
});
