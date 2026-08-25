import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  importBook: vi.fn(),
  importBookFromLocalPath: vi.fn()
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks
}));

// Messages collapse to their keys so assertions can name the exact string the
// composable chose without depending on locale wording.
vi.mock('@/i18n', () => ({
  t: (key: string) => key
}));

import { useImportBook, type ImportUnit } from './useImportBook';

function bookFile(name: string): File {
  return new File(['content'], name, { type: 'text/plain' });
}

// A microtask/macrotask flush: the submit loop advances one file per awaited
// executor, so draining the queue lets each in-flight import settle.
function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('useImportBook', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Acceptance (a): the N / M counter advances as each file completes.
  it('advances the N / M progress as each file finishes', async () => {
    const resolvers: Array<(value: { id: string }) => void> = [];
    mocks.importBook.mockImplementation(
      () => new Promise((resolve) => { resolvers.push(resolve); })
    );

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt'), bookFile('b.txt'), bookFile('c.txt')]);
    const done = book.submitFiles('/');

    // Only the first file is in flight.
    expect(book.progress.value.total).toBe(3);
    expect(book.progress.value.current).toBe(1);
    expect(book.progress.value.filename).toBe('a.txt');
    expect(book.progress.value.percentage).toBe(0);
    expect(mocks.importBook).toHaveBeenCalledTimes(1);

    resolvers[0]({ id: 'id-a' });
    await flush();
    expect(book.progress.value.completed).toBe(1);
    expect(book.progress.value.current).toBe(2);
    expect(mocks.importBook).toHaveBeenCalledTimes(2);

    resolvers[1]({ id: 'id-b' });
    await flush();
    expect(book.progress.value.current).toBe(3);

    resolvers[2]({ id: 'id-c' });
    const result = await done;

    expect(result?.successCount).toBe(3);
    expect(book.progress.value.completed).toBe(3);
    expect(book.progress.value.percentage).toBe(100);
    expect(book.success.value).toBe('libraryForms.importBook.results.many');
  });

  // Acceptance (b): abort stops at a file boundary, sends no further requests,
  // and the untouched remainder is cancelled — never failed.
  it('stops sending after abort and marks the remainder cancelled', async () => {
    const resolvers: Array<(value: { id: string }) => void> = [];
    mocks.importBook.mockImplementation(
      () => new Promise((resolve) => { resolvers.push(resolve); })
    );

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt'), bookFile('b.txt'), bookFile('c.txt')]);
    const done = book.submitFiles('/');

    resolvers[0]({ id: 'id-a' });
    await flush();
    expect(mocks.importBook).toHaveBeenCalledTimes(2);

    // Abort while the second file is in flight; it finishes, the third never starts.
    book.abort();
    expect(book.cancelRequested.value).toBe(true);

    resolvers[1]({ id: 'id-b' });
    const result = await done;

    expect(mocks.importBook).toHaveBeenCalledTimes(2);
    expect(book.files.value[2].status).toBe('cancelled');
    expect(book.files.value.some((item) => item.status === 'failed')).toBe(false);
    expect(result?.successCount).toBe(2);
    expect(result?.cancelledCount).toBe(1);
    expect(result?.failedCount).toBe(0);
    expect(result?.cancelled).toBe(true);
    // A cancel with prior successes reuses the partial-success wording, with no
    // failure banner.
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('');
  });

  // Acceptance (c): a real failure among successes aggregates to partial success.
  it('aggregates a partial-success result when one file fails', async () => {
    mocks.importBook
      .mockResolvedValueOnce({ id: 'id-a' })
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ id: 'id-c' });

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt'), bookFile('b.txt'), bookFile('c.txt')]);
    const result = await book.submitFiles('/');

    expect(result?.successCount).toBe(2);
    expect(result?.failedCount).toBe(1);
    expect(result?.cancelled).toBe(false);
    expect(result?.firstImportedId).toBe('id-a');
    expect(book.files.value[1].status).toBe('failed');
    expect(book.files.value[1].error).toBe('boom');
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('libraryForms.importBook.errors.someFailed');
  });

  // Reverse condition: a single-file import keeps its original wording and does
  // not gain a batch progress bar.
  it('leaves single-file import wording unchanged', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-a' });

    const book = useImportBook();
    book.setBookFiles([bookFile('solo.txt')]);
    const result = await book.submitFiles('/');

    expect(result?.total).toBe(1);
    expect(result?.successCount).toBe(1);
    expect(result?.cancelled).toBe(false);
    expect(book.success.value).toBe('libraryForms.importBook.results.one');
    // total === 1, so the modal's showProgress (total > 1) never lights up.
    expect(book.progress.value.total).toBe(1);
  });

  // A signal-honouring executor that bails out on abort is a cancellation of the
  // in-flight unit, not a failure — it must not report someFailed.
  it('classifies an abort-driven executor rejection as cancelled', async () => {
    const executor = vi.fn((unit: ImportUnit, signal: AbortSignal) =>
      unit.filename === 'a'
        ? Promise.resolve('id-a')
        : new Promise<string>((_, reject) => {
            signal.addEventListener('abort', () => reject(new Error('aborted')));
          })
    );

    const book = useImportBook();
    const done = book.submit(
      [{ filename: 'a', title: 'A' }, { filename: 'b', title: 'B' }],
      executor
    );

    // Let 'a' resolve and 'b' start before aborting mid-flight.
    await flush();
    book.abort();
    const result = await done;

    expect(result?.successCount).toBe(1);
    expect(result?.failedCount).toBe(0);
    expect(result?.cancelledCount).toBe(1);
    expect(result?.cancelled).toBe(true);
    expect(book.files.value[1].status).toBe('cancelled');
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('');
  });

  // Desktop host-path import reuses the same N / M loop as the upload path: one
  // Wails call per file, progress advancing as each finishes, and the display
  // name is the basename of the host path rather than the whole path.
  it('advances the N / M progress for desktop host paths', async () => {
    const resolvers: Array<(value: { id: string; path: string }) => void> = [];
    mocks.importBookFromLocalPath.mockImplementation(
      () => new Promise((resolve) => { resolvers.push(resolve); })
    );

    const book = useImportBook();
    const done = book.submitLocalPaths(
      ['/books/a.txt', '/books/b.txt', '/books/c.txt'],
      '/'
    );

    // Only the first path is in flight, named by its basename.
    expect(book.progress.value.total).toBe(3);
    expect(book.progress.value.current).toBe(1);
    expect(book.progress.value.filename).toBe('a.txt');
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(1);
    expect(mocks.importBookFromLocalPath).toHaveBeenLastCalledWith('/books/a.txt', '/');
    // The upload executor is never touched by the desktop path.
    expect(mocks.importBook).not.toHaveBeenCalled();

    resolvers[0]({ id: 'id-a', path: '/books/a.txt' });
    await flush();
    expect(book.progress.value.completed).toBe(1);
    expect(book.progress.value.current).toBe(2);
    expect(book.progress.value.filename).toBe('b.txt');

    resolvers[1]({ id: 'id-b', path: '/books/b.txt' });
    await flush();
    expect(book.progress.value.current).toBe(3);

    resolvers[2]({ id: 'id-c', path: '/books/c.txt' });
    const result = await done;

    expect(result?.successCount).toBe(3);
    expect(result?.firstImportedId).toBe('id-a');
    expect(book.files.value.map((item) => item.status)).toEqual(['success', 'success', 'success']);
    expect(book.progress.value.percentage).toBe(100);
  });

  // A per-file desktop import failure comes back as the result's Error field, not
  // a rejected call, and must land as a failed unit among the successes.
  it('reports a per-file desktop import error as a failed unit', async () => {
    mocks.importBookFromLocalPath
      .mockResolvedValueOnce({ id: 'id-a', path: '/books/a.txt' })
      .mockResolvedValueOnce({ path: '/books/b.txt', error: 'unsupported file' });

    const book = useImportBook();
    const result = await book.submitLocalPaths(['/books/a.txt', '  ', '/books/b.txt'], '/sci-fi');

    // The blank path is filtered before any call, mirroring the Go picker's
    // normalizeSelectedLocalPaths, so only the two real paths import.
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(2);
    expect(result?.total).toBe(2);
    expect(result?.successCount).toBe(1);
    expect(result?.failedCount).toBe(1);
    expect(book.files.value[1].status).toBe('failed');
    expect(book.files.value[1].error).toBe('unsupported file');
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('libraryForms.importBook.errors.someFailed');
  });

  // The import unit is no longer bound to File: submit drives any executor, and
  // the upload path is only one of them.
  it('drives an arbitrary executor without touching File uploads', async () => {
    const executor = vi.fn(async (unit: ImportUnit) => `id-${unit.filename}`);
    const units: ImportUnit[] = [
      { filename: 'x', title: 'X' },
      { filename: 'y', title: 'Y' }
    ];

    const book = useImportBook();
    const result = await book.submit(units, executor);

    expect(mocks.importBook).not.toHaveBeenCalled();
    expect(executor).toHaveBeenCalledTimes(2);
    expect(result?.successCount).toBe(2);
    expect(result?.firstImportedId).toBe('id-x');
    expect(book.files.value.map((item) => item.status)).toEqual(['success', 'success']);
  });
});
