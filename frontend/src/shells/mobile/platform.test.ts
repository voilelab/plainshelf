import { beforeEach, expect, it, vi } from 'vitest';
const mocks = vi.hoisted(() => ({ native: false, writeFile: vi.fn(), open: vi.fn(), content: vi.fn() }));
vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => mocks.native } }));
vi.mock('@capacitor/browser', () => ({ Browser: { open: mocks.open } }));
vi.mock('@capacitor/filesystem', () => ({
  Directory: { Documents: 'DOCUMENTS' }, Encoding: { UTF8: 'utf8' },
  Filesystem: { writeFile: mocks.writeFile }
}));
vi.mock('@/providers', () => ({ getBookshelfProvider: () => ({ downloadBookContent: mocks.content }) }));
vi.mock('@/providers/platformCapabilities', () => ({ createBrowserPlatform: () => ({}) }));
import { createMobilePlatform } from './platform';
beforeEach(() => { vi.clearAllMocks(); mocks.native = false; });
it('keeps browser preview downloads on the browser path', () => {
  expect(createMobilePlatform().saveBookContentToFile).toBeUndefined();
  expect(createMobilePlatform().coversMustBeFetchedAsBlob).toBe(true);
});
it('exports through the active shelf reader to shared Documents', async () => {
  mocks.native = true;
  mocks.content.mockResolvedValue(new Blob(['book body']));
  await expect(createMobilePlatform().saveBookContentToFile!('b1', 'My Book.txt'))
    .resolves.toBe('Documents/PlainShelf/My Book.txt');
  expect(mocks.content).toHaveBeenCalledWith('b1');
  expect(mocks.writeFile).toHaveBeenCalledWith({
    path: 'PlainShelf/My Book.txt', data: 'book body', directory: 'DOCUMENTS', encoding: 'utf8', recursive: true
  });
});
it('opens authorization in the native browser', async () => {
  mocks.native = true;
  await createMobilePlatform().openExternalUrl!('https://example.com/auth');
  expect(mocks.open).toHaveBeenCalledWith({ url: 'https://example.com/auth' });
});
