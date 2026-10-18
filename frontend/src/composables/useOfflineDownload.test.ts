import { beforeEach, expect, it, vi } from 'vitest';
const mocks = vi.hoisted(() => ({ enabled: false, downloadBook: vi.fn(), removeDownload: vi.fn(), getDownloadState: vi.fn() }));
vi.mock('@/providers', () => ({ getBookshelfProvider: () => mocks }));
vi.mock('./useShelfCapabilities', () => ({ useShelfCapabilities: () => ({ download: { get value() { return mocks.enabled; } } }) }));
import { useOfflineDownload } from './useOfflineDownload';
beforeEach(() => { vi.clearAllMocks(); mocks.enabled = false; });
it('does not invoke offline methods when the shelf disables downloads', async () => {
  const download = useOfflineDownload(() => 'b1');
  expect(download.supported.value).toBe(false);
  await download.refresh();
  await download.download();
  await download.remove();
  expect(mocks.getDownloadState).not.toHaveBeenCalled();
  expect(mocks.downloadBook).not.toHaveBeenCalled();
  expect(mocks.removeDownload).not.toHaveBeenCalled();
});
it('uses the offline interface when the owning layer advertises downloads', async () => {
  mocks.enabled = true;
  const download = useOfflineDownload(() => 'b1');
  expect(download.supported.value).toBe(true);
  await download.download();
  expect(mocks.downloadBook).toHaveBeenCalledWith('b1');
  expect(download.state.value).toBe('downloaded');
});
