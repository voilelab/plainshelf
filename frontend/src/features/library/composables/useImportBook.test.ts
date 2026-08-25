import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  importBook: vi.fn()
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
