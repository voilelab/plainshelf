import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  importBook: vi.fn(),
  importBookFromLocalPath: vi.fn(),
  createSource: vi.fn(),
  getBookContent: vi.fn()
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => mocks,
  getBookshelfProvider: () => mocks
}));

vi.mock('@/i18n', () => ({
  t: (key: string) => key
}));

import { useImportBook, type ImportUnit } from './useImportBook';

function bookFile(name: string): File {
  return new File(['content'], name, { type: 'text/plain' });
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('useImportBook', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('advances the N / M progress as each file finishes', async () => {
    const resolvers: Array<(value: { id: string }) => void> = [];
    mocks.importBook.mockImplementation(
      () => new Promise((resolve) => { resolvers.push(resolve); })
    );

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt'), bookFile('b.txt'), bookFile('c.txt')]);
    const done = book.submitFiles('/');

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
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('');
  });

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

  it('leaves single-file import wording unchanged', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-a' });

    const book = useImportBook();
    book.setBookFiles([bookFile('solo.txt')]);
    const result = await book.submitFiles('/');

    expect(result?.total).toBe(1);
    expect(result?.successCount).toBe(1);
    expect(result?.cancelled).toBe(false);
    expect(book.success.value).toBe('libraryForms.importBook.results.one');
    expect(book.progress.value.total).toBe(1);
  });

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

    expect(book.progress.value.total).toBe(3);
    expect(book.progress.value.current).toBe(1);
    expect(book.progress.value.filename).toBe('a.txt');
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(1);
    expect(mocks.importBookFromLocalPath).toHaveBeenLastCalledWith('/books/a.txt', '/');
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

  it('reports a per-file desktop import error as a failed unit', async () => {
    mocks.importBookFromLocalPath
      .mockResolvedValueOnce({ id: 'id-a', path: '/books/a.txt' })
      .mockResolvedValueOnce({ path: '/books/b.txt', error: 'unsupported file' });

    const book = useImportBook();
    const result = await book.submitLocalPaths(['/books/a.txt', '  ', '/books/b.txt'], '/sci-fi');

    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(2);
    expect(result?.total).toBe(2);
    expect(result?.successCount).toBe(1);
    expect(result?.failedCount).toBe(1);
    expect(book.files.value[1].status).toBe('failed');
    expect(book.files.value[1].error).toBe('unsupported file');
    expect(book.success.value).toBe('libraryForms.importBook.results.partial');
    expect(book.error.value).toBe('libraryForms.importBook.errors.someFailed');
  });

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

  it('offers a chapter conversion when a single imported TXT reads as chaptered', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-novel' });
    mocks.getBookContent.mockResolvedValue({ content: '第一章 起\n正文\n第二章 承' });

    const book = useImportBook();
    book.setBookFiles([bookFile('novel.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    expect(mocks.getBookContent).toHaveBeenCalledWith('id-novel');
    expect(book.chapterSuggestion.value).toEqual({
      bookId: 'id-novel',
      chapters: 2,
      content: '第一章 起\n正文\n第二章 承'
    });
  });

  it('converts the detected book into a new current Markdown source', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-novel' });
    mocks.getBookContent.mockResolvedValue({ content: '第一章 起\n正文\n第二章 承' });
    mocks.createSource.mockResolvedValue({ id: 'src-md' });

    const book = useImportBook();
    book.setBookFiles([bookFile('novel.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    const converted = await book.applyChapterConversion();

    expect(converted).toBe(true);
    expect(mocks.createSource).toHaveBeenCalledTimes(1);
    const [convertedBookId, options] = mocks.createSource.mock.calls[0];
    expect(convertedBookId).toBe('id-novel');
    expect(options.format).toBe('md');
    expect(options.setCurrent).toBe(true);
    expect(options.content).toContain('## 第一章 起');
    expect(options.content).toContain('## 第二章 承');
    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('offers no conversion when the imported TXT has no chapter lines', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-plain' });
    mocks.getBookContent.mockResolvedValue({ content: '只是一段散文。\nJust prose.' });

    const book = useImportBook();
    book.setBookFiles([bookFile('plain.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    expect(mocks.getBookContent).toHaveBeenCalledWith('id-plain');
    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('does not probe content for a non-TXT import', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-md' });

    const book = useImportBook();
    book.setBookFiles([bookFile('doc.md')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    expect(mocks.getBookContent).not.toHaveBeenCalled();
    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('offers no conversion for a multi-file import', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-x' });
    mocks.getBookContent.mockResolvedValue({ content: '第一章 起' });

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt'), bookFile('b.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    expect(mocks.getBookContent).not.toHaveBeenCalled();
    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('stays silent when the content probe fails', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-novel' });
    mocks.getBookContent.mockRejectedValue(new Error('network'));

    const book = useImportBook();
    book.setBookFiles([bookFile('novel.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));

    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('clears the suggestion when dismissed', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-novel' });
    mocks.getBookContent.mockResolvedValue({ content: '第一章 起\n第二章 承' });

    const book = useImportBook();
    book.setBookFiles([bookFile('novel.txt')]);
    await book.detectChapterConversion(await book.submitFiles('/'));
    expect(book.chapterSuggestion.value).not.toBeNull();

    book.dismissChapterSuggestion();
    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('discards a detection result when reset mid-probe', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-novel' });
    let resolveContent: (value: { content: string }) => void = () => {};
    mocks.getBookContent.mockImplementation(
      () => new Promise((resolve) => { resolveContent = resolve; })
    );

    const book = useImportBook();
    book.setBookFiles([bookFile('novel.txt')]);
    const result = await book.submitFiles('/');
    const detecting = book.detectChapterConversion(result);

    book.reset();
    resolveContent({ content: '第一章 起\n第二章 承' });
    await detecting;

    expect(book.chapterSuggestion.value).toBeNull();
  });

  it('clears the retained selection without wiping the result message', async () => {
    mocks.importBook.mockResolvedValue({ id: 'id-a' });

    const book = useImportBook();
    book.setBookFiles([bookFile('a.txt')]);
    await book.submitFiles('/');
    expect(book.files.value.length).toBe(1);
    expect(book.success.value).toBe('libraryForms.importBook.results.one');

    book.clearImportSelection();
    expect(book.files.value.length).toBe(0);
    expect(book.success.value).toBe('libraryForms.importBook.results.one');
  });
});

vi.mock('@/providers/platformCapabilities', () => ({ getPlatformCapabilities: () => mocks }));
