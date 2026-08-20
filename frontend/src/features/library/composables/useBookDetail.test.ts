import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  getBook: vi.fn(),
  getReadProgress: vi.fn(),
  getSource: vi.fn(),
  getBookContent: vi.fn(),
  deleteBook: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => mocks,
  bookshelfWriter: () => mocks
}));

import { useBookDetail } from './useBookDetail';

describe('useBookDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBook.mockResolvedValue({
      id: 'book-1',
      title: 'Book',
      format: 'txt',
      current_source: 'source-1'
    });
    mocks.getReadProgress.mockResolvedValue({ char_offset: 0, percent: 0 });
    mocks.getSource.mockResolvedValue({ id: 'source-1', format: 'txt' });
    mocks.getBookContent.mockResolvedValue({ content: 'body' });
  });

  it('loads the book together with its current source', async () => {
    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.book.value?.id).toBe('book-1');
    expect(detail.currentSource.value?.id).toBe('source-1');
    expect(detail.error.value).toBe('');
  });

  // A shelf edited by hand or by a sync tool can leave current_source naming a
  // source that is gone. That must not cost the page its title and cover.
  it('still shows the book when the current source cannot be fetched', async () => {
    mocks.getSource.mockRejectedValue(new Error('source not found'));

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.book.value?.id).toBe('book-1');
    expect(detail.currentSource.value).toBeNull();
    expect(detail.error.value).toBe('');
  });

  it('still shows the book when the content length cannot be measured', async () => {
    mocks.getReadProgress.mockResolvedValue({ char_offset: 120 });
    mocks.getBookContent.mockRejectedValue(new Error('source not found'));

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.book.value?.id).toBe('book-1');
    expect(detail.progressContentLength.value).toBeNull();
    expect(detail.error.value).toBe('');
  });

  // The chapter card and the percentage both want the same text, and reading a
  // whole book off a network shelf is the expensive part of opening this page.
  it('reads the content once for both the chapter list and the percentage', async () => {
    mocks.getBook.mockResolvedValue({
      id: 'book-1',
      title: 'Book',
      format: 'md',
      current_source: 'source-1'
    });
    mocks.getSource.mockResolvedValue({ id: 'source-1', format: 'md' });
    mocks.getReadProgress.mockResolvedValue({ char_offset: 12 });
    mocks.getBookContent.mockResolvedValue({ content: '## One\nBody\n## Two\nMore' });

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(mocks.getBookContent).toHaveBeenCalledTimes(1);
    expect(detail.chapters.value.map((chapter) => chapter.title)).toEqual(['One', 'Two']);
    expect(detail.progressContentLength.value).toBe('## One\nBody\n## Two\nMore'.length);
  });

  it('does not read the content for a txt book that already knows its percentage', async () => {
    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(mocks.getBookContent).not.toHaveBeenCalled();
    expect(detail.chapters.value).toEqual([]);
  });

  // The source's own format is authoritative for what this build writes, so a
  // book record left at txt must not hide the chapters of a Markdown source.
  it('follows the source format over the book format', async () => {
    mocks.getSource.mockResolvedValue({ id: 'source-1', format: 'md' });
    mocks.getBookContent.mockResolvedValue({ content: '## Only\nBody' });

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.chapters.value).toEqual([{ index: 0, title: 'Only' }]);
  });

  it('lists no chapters for Markdown without a single H2', async () => {
    mocks.getBook.mockResolvedValue({ id: 'book-1', title: 'Book', format: 'md' });
    mocks.getBookContent.mockResolvedValue({ content: '# Title\nJust prose.' });

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.chapters.value).toEqual([]);
    expect(detail.error.value).toBe('');
  });

  // Losing the text costs the chapter card and nothing else on the page.
  it('still shows the book when the chapter text cannot be read', async () => {
    mocks.getBook.mockResolvedValue({
      id: 'book-1',
      title: 'Book',
      format: 'md',
      current_source: 'source-1'
    });
    mocks.getSource.mockResolvedValue({ id: 'source-1', format: 'md' });
    mocks.getBookContent.mockRejectedValue(new Error('source not found'));

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.book.value?.id).toBe('book-1');
    expect(detail.chapters.value).toEqual([]);
    expect(detail.error.value).toBe('');
  });

  // The book itself failing is a real error and still has to surface.
  it('reports an error when the book itself cannot be fetched', async () => {
    mocks.getBook.mockRejectedValue(new Error('book not found'));

    const detail = useBookDetail(() => 'book-1');
    await detail.fetchDetail();

    expect(detail.book.value).toBeNull();
    expect(detail.error.value).toBe('book not found');
  });
});
