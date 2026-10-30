import { beforeEach, expect, it, vi } from 'vitest';
const api = vi.hoisted(() => ({getFolders: vi.fn(), getFolderMeta: vi.fn()}));
vi.mock('@/api/folders', () => ({...api, transferFolder: vi.fn()}));
const { ServerBookshelfProvider } = await import('./serverBookshelfProvider');
beforeEach(() => {
  vi.clearAllMocks();
  api.getFolders.mockResolvedValue(['A','A/x','B','B/y']);
});
it('keeps destination subtrees together using the target shelf metadata', async () => {
  api.getFolderMeta.mockResolvedValue({folders:{B:{sort_key:0,version:''}}});
  expect(await new ServerBookshelfProvider().listShelfFolders('target')).toEqual(['B','B/y','A','A/x']);
  expect(api.getFolderMeta).toHaveBeenCalledExactlyOnceWith('target');
});
it('keeps destinations available when optional target metadata fails', async () => {
  api.getFolderMeta.mockRejectedValue(new Error('optional metadata unavailable'));
  expect(await new ServerBookshelfProvider().listShelfFolders('target')).toEqual(['A','A/x','B','B/y']);
});
