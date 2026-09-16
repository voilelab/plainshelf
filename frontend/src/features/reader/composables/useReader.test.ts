import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  getBook: vi.fn(),
  getBookContent: vi.fn(),
  getSource: vi.fn(),
  getSourceContent: vi.fn(),
  getReadProgress: vi.fn(),
  addReadHistory: vi.fn(),
  saveReadProgress: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => mocks,
  bookshelfWriter: vi.fn()
}));

import { useReader } from './useReader';

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

/** Two macrotask hops, which is longer than any microtask chain under test. */
function settle(): Promise<void> {
  return new Promise((done) => setTimeout(done)).then(() => new Promise((done) => setTimeout(done)));
}

describe('useReader source consistency', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'txt', current_source: 'source-1' });
    mocks.getBookContent.mockResolvedValue({ content: 'content from a different current source' });
    mocks.getSource.mockResolvedValue({ id: 'source-1', schema_version: 1, format: 'md' });
    mocks.getSourceContent.mockResolvedValue('## Source one\nBody');
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.addReadHistory.mockResolvedValue(undefined);
    mocks.saveReadProgress.mockResolvedValue(undefined);
  });

  it('loads content by the source ID used for metadata and asset scope', async () => {
    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(mocks.getSourceContent).toHaveBeenCalledWith('book-1', 'source-1');
    expect(mocks.getBookContent).not.toHaveBeenCalled();
    expect(reader.currentSourceId.value).toBe('source-1');
    expect(reader.content.value).toBe('## Source one\nBody');
    expect(reader.sections.value.map((section) => section.title)).toEqual(['Source one']);
  });

  // A shelf edited by hand or by a sync tool can leave current_source naming a
  // source that is gone. The book-scoped route answers from the newest source
  // the book still has, so the reader must fall back to it rather than fail.
  it('falls back to the book content when the current source is gone', async () => {
    mocks.getSourceContent.mockRejectedValue(new Error('source not found'));
    mocks.getSource.mockRejectedValue(new Error('source not found'));

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(mocks.getBookContent).toHaveBeenCalledWith('book-1');
    expect(reader.content.value).toBe('content from a different current source');
    expect(reader.error.value).toBe('');
  });
});

describe('useReader section navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'md', current_source: 'source-1' });
    mocks.getSource.mockResolvedValue({ id: 'source-1', format: 'md' });
    mocks.getSourceContent.mockResolvedValue('## One\nBody\n## Two\nMore');
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.addReadHistory.mockResolvedValue(undefined);
    mocks.saveReadProgress.mockResolvedValue(undefined);
  });

  it('opens the requested chapter and its offset', async () => {
    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();
    await reader.goToSection(1);

    expect(reader.currentSectionIndex.value).toBe(1);
    expect(reader.progress.value?.char_offset).toBe('## One\nBody\n'.length);
  });

  // A `?section=` deep link is a URL: it can name a chapter this book does not
  // have, and the nearest chapter is a better answer than an error page.
  it('clamps an out-of-range index to the nearest chapter', async () => {
    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    await reader.goToSection(99);
    expect(reader.currentSectionIndex.value).toBe(1);

    await reader.goToSection(-4);
    expect(reader.currentSectionIndex.value).toBe(0);
    expect(reader.progress.value?.char_offset).toBe(0);
  });
});

/*
A route-param change reuses the reader component, so two fetches can be in
flight for two different books at once. Everything below is about what the
second one owes the first: not letting it win, and not losing its position.
*/
describe('useReader overlapping fetches', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBookContent.mockImplementation((bookID: string) =>
      Promise.resolve({ content: `content of ${bookID}` })
    );
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.addReadHistory.mockResolvedValue(undefined);
    mocks.saveReadProgress.mockResolvedValue(undefined);
  });

  // The older fetch is parked inside the parallel read, which is past the
  // guard that follows getBook — so only the guard after Promise.all can stop
  // it painting the previous book over the one now on screen.
  it('keeps the newer book when an older fetch settles later', async () => {
    const olderProgress = deferred<{ char_offset: number; percent: number }>();
    mocks.getBook.mockImplementation((bookID: string) =>
      Promise.resolve({
        id: bookID,
        title: bookID === 'book-1' ? 'Older' : 'Newer',
        format: 'txt',
        current_source: ''
      })
    );
    mocks.getReadProgress.mockImplementation((bookID: string) =>
      bookID === 'book-1' ? olderProgress.promise : Promise.resolve({ char_offset: 0, percent: 0 })
    );

    let requested = 'book-1';
    const reader = useReader(() => requested);

    const older = reader.fetchReaderData();
    await vi.waitFor(() => expect(mocks.getReadProgress).toHaveBeenCalledWith('book-1'));

    requested = 'book-2';
    await reader.fetchReaderData();
    expect(reader.title.value).toBe('Newer');

    olderProgress.resolve({ char_offset: 0, percent: 0 });
    await older;

    expect(reader.title.value).toBe('Newer');
    expect(reader.content.value).toBe('content of book-2');
    expect(reader.loading.value).toBe(false);
    expect(reader.error.value).toBe('');
  });

  // The autosave baseline holds one book at a time, so the previous book's
  // position has to reach storage before the next book replaces it.
  it('persists the previous book before taking the next book baseline', async () => {
    mocks.getBook.mockImplementation((bookID: string) =>
      Promise.resolve({ id: bookID, title: bookID, format: 'md', current_source: '' })
    );
    mocks.getBookContent.mockResolvedValue({ content: '## One\nBody\n## Two\nMore' });

    let requested = 'book-1';
    const reader = useReader(() => requested);
    await reader.fetchReaderData();

    const save = deferred<void>();
    mocks.saveReadProgress.mockReturnValueOnce(save.promise);
    await reader.goToSection(1);

    requested = 'book-2';
    const next = reader.fetchReaderData();
    await settle();

    expect(mocks.saveReadProgress).toHaveBeenCalledWith('book-1', expect.objectContaining({
      char_offset: '## One\nBody\n'.length
    }));
    expect(mocks.getBook).not.toHaveBeenCalledWith('book-2');

    save.resolve();
    await next;
    expect(mocks.getBook).toHaveBeenCalledWith('book-2');
  });
});

/*
Reading is the point; the bookkeeping around it is not. Each case here breaks
one thing the reader does not need and asserts the book still opens.
*/
describe('useReader tolerated failures', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'txt', current_source: 'source-1' });
    mocks.getBookContent.mockResolvedValue({ content: 'book content' });
    mocks.getSource.mockResolvedValue({ id: 'source-1', schema_version: 1, format: 'md' });
    mocks.getSourceContent.mockResolvedValue('## Source one\nBody');
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.addReadHistory.mockResolvedValue(undefined);
    mocks.saveReadProgress.mockResolvedValue(undefined);
  });

  it('opens the book when the read history cannot be recorded', async () => {
    mocks.addReadHistory.mockRejectedValue(new Error('history is full'));

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(reader.content.value).toBe('## Source one\nBody');
    expect(reader.error.value).toBe('');
    expect(reader.loading.value).toBe(false);
  });

  // The source's own metadata carries the format the reader renders with. When
  // only that read fails the content is still there, so the book's format is a
  // better answer than an error page.
  it('falls back to the book format when the source metadata is unreadable', async () => {
    mocks.getSource.mockRejectedValue(new Error('meta.json is gone'));

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(reader.bookFormat.value).toBe('txt');
    expect(reader.sections.value).toHaveLength(1);
    expect(reader.error.value).toBe('');
  });

  it('asks only for the book content when the book names no current source', async () => {
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'txt' });

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(mocks.getSourceContent).not.toHaveBeenCalled();
    expect(mocks.getSource).not.toHaveBeenCalled();
    expect(reader.currentSourceId.value).toBe('');
    expect(reader.content.value).toBe('book content');
  });

  it('reports the failure when the book itself cannot be read', async () => {
    mocks.getBook.mockRejectedValue(new Error('shelf is initializing'));

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(reader.error.value).toBe('shelf is initializing');
    expect(reader.loading.value).toBe(false);
  });
});

describe('useReader chapter boundaries', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'md', current_source: '' });
    mocks.getBookContent.mockResolvedValue({ content: '## One\nBody\n## Two\nMore' });
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.addReadHistory.mockResolvedValue(undefined);
    mocks.saveReadProgress.mockResolvedValue(undefined);
  });

  // What the clamp promises is only the index: an out-of-range section — from a
  // `?section=` link, say — opens the nearest one instead of failing. It does
  // not make the call free, because goToSection still moves the saved position
  // to that chapter's start. Refusing to make the call at all is the page's
  // job, and ReaderView.test.ts is where that is pinned.
  it('opens the nearest chapter when asked to step past either end', async () => {
    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    await reader.goPrevSection();
    expect(reader.currentSectionIndex.value).toBe(0);

    await reader.goToSection(1);
    await reader.goNextSection();
    expect(reader.currentSectionIndex.value).toBe(1);
  });

  // A finished book restores at an offset equal to its length, which is the
  // one offset that falls outside every half-open section range.
  it('restores a position at the very end into the last chapter', async () => {
    const content = '## One\nBody\n## Two\nMore';
    mocks.getReadProgress.mockResolvedValue({ char_offset: content.length, percent: 100 });

    const reader = useReader(() => 'book-1');
    await reader.fetchReaderData();

    expect(reader.currentSectionIndex.value).toBe(1);
    expect(reader.progress.value?.char_offset).toBe(content.length);
  });
});
