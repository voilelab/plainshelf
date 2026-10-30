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

const { createMobileDeviceDocumentStorage } = await import('./deviceDocumentStorage');

const PATH = 'plainshelf-cache/doc.json';

describe('createMobileDeviceDocumentStorage', () => {
  beforeEach(() => {
    readFileMock.mockReset();
    writeFileMock.mockReset().mockResolvedValue(undefined);
  });

  it('reads and writes the app-private document', async () => {
    const stored = '{"version":1}';
    readFileMock.mockResolvedValue({ data: stored });

    const storage = createMobileDeviceDocumentStorage(PATH);

    expect(await storage.load()).toBe(stored);
    expect(readFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: PATH, directory: 'DATA' })
    );

    await storage.save(stored);
    expect(writeFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: PATH, data: stored, recursive: true })
    );
  });

  it('treats a missing file as nothing stored yet', async () => {
    readFileMock.mockRejectedValue(new Error('File does not exist.'));

    expect(await createMobileDeviceDocumentStorage(PATH).load()).toBeNull();
  });

  it('propagates a read failure that is not a missing file', async () => {
    readFileMock.mockRejectedValue(new Error('EACCES: permission denied'));

    await expect(createMobileDeviceDocumentStorage(PATH).load()).rejects.toThrow('permission denied');
  });
});
