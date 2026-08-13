import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  getBook: vi.fn(),
  listSources: vi.fn(),
  getSourceContent: vi.fn(),
  updateSourceContent: vi.fn(),
  setCurrentSource: vi.fn(),
  createSource: vi.fn(),
  deleteSource: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => mocks,
  bookshelfWriter: () => mocks
}));

import { useSourceEditorSession } from './useSourceEditorSession';

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

function source(id: string, format: 'txt' | 'md' = 'txt') {
  return { id, format, created_at: '', comment: '', md5_hash: id };
}

describe('useSourceEditorSession', () => {
  let currentBookId: string;

  beforeEach(() => {
    vi.clearAllMocks();
    currentBookId = 'book-1';
    mocks.getBook.mockResolvedValue({
      id: 'book-1',
      title: 'Book',
      format: 'txt',
      current_source: 'source-2'
    });
    mocks.listSources.mockResolvedValue([source('source-1'), source('source-2')]);
    mocks.getSourceContent.mockImplementation(async (_bookId: string, sourceId: string) => `content:${sourceId}`);
    mocks.updateSourceContent.mockResolvedValue(undefined);
    mocks.setCurrentSource.mockResolvedValue(undefined);
    mocks.createSource.mockResolvedValue(source('source-3'));
    mocks.deleteSource.mockResolvedValue(undefined);
  });

  it('loads the current source and falls back to the first source when needed', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();

    expect(session.activeSourceId.value).toBe('source-2');
    expect(session.content.value).toBe('content:source-2');

    mocks.getBook.mockResolvedValue({
      id: 'book-1',
      title: 'Book',
      format: 'txt',
      current_source: 'missing'
    });
    await session.fetchInitial();
    expect(session.activeSourceId.value).toBe('source-1');
  });

  it('keeps a newer source load when an older request settles later', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    const slow = deferred<string>();
    const fast = deferred<string>();
    mocks.getSourceContent.mockImplementation((_bookId: string, sourceId: string) => {
      if (sourceId === 'source-1') return slow.promise;
      if (sourceId === 'source-2') return fast.promise;
      return Promise.resolve('');
    });

    const olderLoad = session.loadSource('source-1');
    const newerLoad = session.loadSource('source-2');
    slow.resolve('stale source');
    await olderLoad;
    expect(session.contentLoading.value).toBe(true);

    fast.resolve('new source');
    await newerLoad;
    expect(session.activeSourceId.value).toBe('source-2');
    expect(session.content.value).toBe('new source');
    expect(session.contentLoading.value).toBe(false);
  });

  it('ignores an old book session after the route changes', async () => {
    const oldBook = deferred<Record<string, unknown>>();
    const oldSources = deferred<ReturnType<typeof source>[]>();
    mocks.getBook.mockImplementation((bookId: string) => {
      if (bookId === 'book-1') return oldBook.promise;
      return Promise.resolve({ id: 'book-2', title: 'New Book', format: 'md', current_source: 'new-source' });
    });
    mocks.listSources.mockImplementation((bookId: string) => {
      if (bookId === 'book-1') return oldSources.promise;
      return Promise.resolve([source('new-source', 'md')]);
    });

    const session = useSourceEditorSession(() => currentBookId, () => true);
    const oldFetch = session.fetchInitial();
    currentBookId = 'book-2';
    await session.fetchInitial();

    oldBook.resolve({ id: 'book-1', title: 'Old Book', format: 'txt', current_source: 'old-source' });
    oldSources.resolve([source('old-source')]);
    await oldFetch;

    expect(session.book.value?.id).toBe('book-2');
    expect(session.activeSourceId.value).toBe('new-source');
    expect(session.content.value).toBe('content:new-source');
  });

  it('saves a snapshot, refreshes metadata, and advances the dirty baseline', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    session.content.value = 'edited';
    await session.save();

    expect(mocks.updateSourceContent).toHaveBeenCalledWith('book-1', 'source-2', 'edited');
    expect(session.isDirty.value).toBe(false);
    expect(session.saveSuccess.value).toBe('Source saved.');
    expect(mocks.listSources).toHaveBeenCalledTimes(2);
  });

  it('creates a regular source and loads it into the editor', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    mocks.listSources.mockResolvedValue([source('source-1'), source('source-2'), source('source-3')]);

    expect(await session.createSource()).toBe(true);
    expect(mocks.createSource).toHaveBeenCalledWith('book-1');
    expect(session.activeSourceId.value).toBe('source-3');
    expect(session.content.value).toBe('content:source-3');
  });

  it('does not let a pending create replace a newer source selection', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    const createdSource = deferred<ReturnType<typeof source>>();
    mocks.createSource.mockReturnValue(createdSource.promise);
    mocks.listSources.mockResolvedValue([source('source-1'), source('source-2'), source('source-3')]);

    const creating = session.createSource();
    await session.loadSource('source-1');
    createdSource.resolve(source('source-3'));

    expect(await creating).toBe(true);
    expect(session.activeSourceId.value).toBe('source-1');
    expect(session.content.value).toBe('content:source-1');
  });

  it('creates a derived source and updates the local current-source metadata', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    mocks.createSource.mockResolvedValue(source('source-3', 'md'));
    mocks.listSources.mockResolvedValue([source('source-1'), source('source-2'), source('source-3', 'md')]);

    const input = { content: '## Chapter', format: 'md' as const, comment: 'Converted', setCurrent: true };
    expect(await session.createDerivedSource(input)).toBe(true);
    expect(mocks.createSource).toHaveBeenCalledWith('book-1', input);
    expect(session.book.value?.current_source).toBe('source-3');
    expect(session.book.value?.format).toBe('md');
    expect(session.saveSuccess.value).toBe('Derived source created.');
  });

  it('deletes the active source and selects the remaining current source', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    mocks.listSources.mockResolvedValue([source('source-1')]);

    expect(await session.deleteSource('source-2')).toBe(true);
    expect(mocks.deleteSource).toHaveBeenCalledWith('book-1', 'source-2');
    expect(session.activeSourceId.value).toBe('source-1');
    expect(session.content.value).toBe('content:source-1');
  });

  it('sets the current source without replacing a newer active source', async () => {
    const session = useSourceEditorSession(() => currentBookId, () => true);
    await session.fetchInitial();
    const request = deferred<void>();
    mocks.setCurrentSource.mockReturnValue(request.promise);

    const setting = session.setCurrentSource();
    await session.loadSource('source-1');
    request.resolve(undefined);
    await setting;

    expect(session.book.value?.current_source).toBe('source-2');
    expect(session.saveSuccess.value).toBe('');
  });
});
