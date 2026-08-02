import { beforeEach, describe, expect, it, vi } from 'vitest';

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
const { MOBILE_READ_HISTORY_PATH, createFilesystemReadHistoryStorage } = await import('./storage');

describe('createFilesystemReadHistoryStorage', () => {
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

  it('treats a missing file as no history yet', async () => {
    readFileMock.mockRejectedValue(new Error('File does not exist.'));

    expect(await createFilesystemReadHistoryStorage().load()).toBeNull();
  });

  // Reporting a failed read as "no history" would let the next write replace a
  // document that was never read, wiping every shelf's history.
  it('propagates a read failure that is not a missing file', async () => {
    readFileMock.mockRejectedValue(new Error('EACCES: permission denied'));

    await expect(createFilesystemReadHistoryStorage().load()).rejects.toThrow('permission denied');
  });

  it('does not overwrite the stored document when the read fails', async () => {
    readFileMock.mockRejectedValue(new Error('EACCES: permission denied'));
    const store = new ReadHistoryStore(createFilesystemReadHistoryStorage());

    await expect(store.add('shelf-1', 'book-1')).rejects.toThrow('permission denied');
    expect(writeFileMock).not.toHaveBeenCalled();
  });
});
