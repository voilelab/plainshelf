import { beforeEach, describe, expect, it, vi } from 'vitest';

// The backend itself is covered by shells/mobile/deviceDocumentStorage.test.ts.
// What this file pins is the read-history wiring on top of them: the
// app-private path, and that a failed read never lets a write through.

const { readFileMock, writeFileMock } = vi.hoisted(() => ({
  readFileMock: vi.fn(),
  writeFileMock: vi.fn()
}));

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: { readFile: readFileMock, writeFile: writeFileMock }
}));

const { ReadHistoryStore } = await import('./index');
const { MOBILE_READ_HISTORY_PATH } = await import('./storage');
const { createMobileDeviceDocumentStorage } = await import('@/shells/mobile/deviceDocumentStorage');

/** What the mobile shell builds for this document. */
const createFilesystemReadHistoryStorage = () =>
  createMobileDeviceDocumentStorage(MOBILE_READ_HISTORY_PATH);

describe('read history on the mobile shell backend', () => {
  beforeEach(() => {
    readFileMock.mockReset();
    writeFileMock.mockReset().mockResolvedValue(undefined);
  });

  it('reads and writes the app-private document', async () => {
    const stored = '{"version":1,"limit":100,"shelves":{"shelf-1":["book-1"]}}';
    readFileMock.mockResolvedValue({ data: stored });

    const storage = createFilesystemReadHistoryStorage();

    expect(await storage.load()).toBe(stored);
    expect(readFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: MOBILE_READ_HISTORY_PATH, directory: 'DATA' })
    );

    await storage.save(stored);
    expect(writeFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: MOBILE_READ_HISTORY_PATH, data: stored, recursive: true })
    );
  });

  // Reporting a failed read as "no history" would let the next write replace a
  // document that was never read, wiping every shelf's history.
  it('does not overwrite the stored document when the read fails', async () => {
    readFileMock.mockRejectedValue(new Error('EACCES: permission denied'));
    const store = new ReadHistoryStore(createFilesystemReadHistoryStorage());

    await expect(store.add('shelf-1', 'book-1')).rejects.toThrow('permission denied');
    expect(writeFileMock).not.toHaveBeenCalled();
  });
});
