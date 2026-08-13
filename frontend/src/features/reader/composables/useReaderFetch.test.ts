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

vi.mock('@/api/settings', () => ({
  getDefaultSplitConfigSetting: vi.fn().mockResolvedValue({ type: 'none' })
}));

vi.mock('@/composables/useWriteAccess', () => ({
  isLibraryEditingSupported: () => false
}));

import { useReader } from './useReader';

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
});
