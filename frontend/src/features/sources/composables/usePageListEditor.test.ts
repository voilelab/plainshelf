import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/api/client';

const mocks = vi.hoisted(() => ({
  getSourcePages: vi.fn(),
  replaceSourcePages: vi.fn(),
  insertSourcePage: vi.fn(),
  deleteSourcePage: vi.fn(),
  setSourcePageDirection: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => mocks,
  bookshelfWriter: () => mocks
}));

import { usePageListEditor } from './usePageListEditor';

function list(names: string[], md5Hash: string) {
  return { pages: names.map((name) => ({ name, missing: false })), unlisted: [], md5_hash: md5Hash };
}

describe('usePageListEditor', () => {
  let changed: ReturnType<typeof vi.fn<() => void>>;

  beforeEach(() => {
    vi.clearAllMocks();
    changed = vi.fn<() => void>();
    mocks.getSourcePages.mockResolvedValue(list(['a.png', 'b.png', 'c.png'], 'h0'));
  });

  async function loaded(writes = true) {
    const editor = usePageListEditor(() => 'book', () => 'src', () => writes, changed);
    await editor.load();
    return editor;
  }

  it('sends the reordered list with the md5 it was read at', async () => {
    mocks.replaceSourcePages.mockResolvedValue(list(['c.png', 'a.png', 'b.png'], 'h1'));
    const editor = await loaded();

    expect(await editor.reorder(2, 0)).toBe(true);

    expect(mocks.replaceSourcePages).toHaveBeenCalledWith('book', 'src', 'h0', ['c.png', 'a.png', 'b.png']);
    expect(editor.pageList.value?.md5_hash).toBe('h1');
    expect(changed).toHaveBeenCalledOnce();
  });

  it('chains each upload on the md5 the previous one returned, after the selected page', async () => {
    mocks.insertSourcePage
      .mockResolvedValueOnce(list(['a.png', 'n1.png', 'b.png', 'c.png'], 'h1'))
      .mockResolvedValueOnce(list(['a.png', 'n1.png', 'n2.png', 'b.png', 'c.png'], 'h2'));
    const editor = await loaded();
    editor.select(0);

    const files = [new Blob(['1'], { type: 'image/png' }), new Blob(['2'], { type: 'image/png' })];
    expect(await editor.addImages(files)).toBe(2);

    expect(mocks.insertSourcePage.mock.calls.map((call) => [call[2], call[3]])).toEqual([['h0', 1], ['h1', 2]]);
    expect(editor.selectedIndex.value).toBe(2);
  });

  it('stops editing on a conflict until the list is reloaded', async () => {
    mocks.replaceSourcePages.mockRejectedValue(new ApiError('changed', { status: 409, code: 'PAGE_LIST_CONFLICT' }));
    const editor = await loaded();

    expect(await editor.reorder(0, 1)).toBe(false);
    expect(editor.conflict.value).toBe(true);
    expect(editor.canEdit.value).toBe(false);
    expect(await editor.removeImage('a.png')).toBe(false);
    expect(mocks.deleteSourcePage).not.toHaveBeenCalled();

    mocks.getSourcePages.mockResolvedValue(list(['b.png', 'a.png'], 'h9'));
    await editor.load();
    expect(editor.conflict.value).toBe(false);
    expect(editor.canEdit.value).toBe(true);
  });

  it('stops a multi-file upload at the first failure', async () => {
    mocks.insertSourcePage.mockRejectedValue(new ApiError('image too large'));
    const editor = await loaded();

    expect(await editor.addImages([new Blob(['1']), new Blob(['2'])])).toBe(0);
    expect(mocks.insertSourcePage).toHaveBeenCalledOnce();
    expect(editor.error.value).toBe('image too large');
  });

  it('does not edit when writes are disabled', async () => {
    const editor = await loaded(false);

    expect(await editor.reorder(0, 1)).toBe(false);
    await editor.setDirection('rtl');
    expect(mocks.replaceSourcePages).not.toHaveBeenCalled();
    expect(mocks.setSourcePageDirection).not.toHaveBeenCalled();
  });

  it('applies the direction the server stored', async () => {
    mocks.setSourcePageDirection.mockResolvedValue({ id: 'src', page_direction: 'rtl' });
    const editor = await loaded();

    await editor.setDirection('rtl');

    expect(mocks.setSourcePageDirection).toHaveBeenCalledWith('book', 'src', 'rtl');
    expect(editor.direction.value).toBe('rtl');
    expect(changed).toHaveBeenCalledOnce();
  });
});
