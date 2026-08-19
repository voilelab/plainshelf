import { beforeEach, describe, expect, it, vi } from 'vitest';

const { rmdirMock, writeFileMock } = vi.hoisted(() => ({
  rmdirMock: vi.fn(),
  writeFileMock: vi.fn()
}));

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: { rmdir: rmdirMock, writeFile: writeFileMock }
}));

const { removeCacheScope, scopeDir, UNSCOPED_DIR_NAME, writeBinaryFile, writeTextFile } =
  await import('./mobileCacheFs');

beforeEach(() => {
  rmdirMock.mockReset().mockResolvedValue(undefined);
  writeFileMock.mockReset().mockResolvedValue({ uri: 'file:///written' });
});

describe('removeCacheScope', () => {
  it('deletes the directory a shelf downloaded into', async () => {
    await removeCacheScope('http://host:20000|main');

    expect(rmdirMock).toHaveBeenCalledWith({
      path: scopeDir('http://host:20000|main'),
      directory: 'DATA',
      recursive: true
    });
  });

  // scopeDir maps an empty key to the shared pre-scope directory, so removing
  // a corrupt entry that derives no identity would take unrelated downloads
  // with it.
  it('refuses an empty scope key rather than wiping the unscoped directory', async () => {
    await removeCacheScope('');

    expect(rmdirMock).not.toHaveBeenCalled();
    expect(scopeDir('')).toContain(UNSCOPED_DIR_NAME);
  });
});

// The web fallback creates a missing parent directory by checking whether it
// exists and then creating it, in separate IndexedDB round trips. Two writes
// into the same new directory - a book's manifest and its cover, or a book and
// the shelf snapshot above it - therefore both see it missing, both create it,
// and the loser fails. Downloading a book must not fall over because another
// write got there first.
describe('writing through the plugin', () => {
  const collision = new Error('Current directory does already exist.');

  it('retries a text write once when a concurrent write created the directory', async () => {
    writeFileMock.mockRejectedValueOnce(collision);

    await writeTextFile('plainshelf-cache/scopes/s/books/b/manifest.json', '{}');

    expect(writeFileMock).toHaveBeenCalledTimes(2);
    expect(writeFileMock.mock.calls[1][0]).toEqual(writeFileMock.mock.calls[0][0]);
    expect(writeFileMock.mock.calls[1][0]).toMatchObject({
      path: 'plainshelf-cache/scopes/s/books/b/manifest.json',
      data: '{}',
      directory: 'DATA',
      encoding: 'utf8',
      recursive: true
    });
  });

  it('retries a binary write on the same collision', async () => {
    writeFileMock.mockRejectedValueOnce(collision);

    await writeBinaryFile('plainshelf-cache/scopes/s/books/b/cover', 'AAAA');

    expect(writeFileMock).toHaveBeenCalledTimes(2);
    expect(writeFileMock.mock.calls[1][0]).toMatchObject({
      path: 'plainshelf-cache/scopes/s/books/b/cover',
      data: 'AAAA',
      directory: 'DATA',
      recursive: true
    });
  });

  it('writes once when nothing collides', async () => {
    await writeTextFile('plainshelf-cache/scopes/s/shelf-snapshot.json', '{}');

    expect(writeFileMock).toHaveBeenCalledTimes(1);
  });

  // Writes report failure to their caller by design: a caller that must know
  // whether it committed still can. Only the directory collision is absorbed.
  it('propagates any other write failure', async () => {
    writeFileMock.mockRejectedValueOnce(new Error('Quota exceeded'));

    await expect(writeTextFile('plainshelf-cache/scopes/s/x.json', '{}')).rejects.toThrow(
      'Quota exceeded'
    );
    expect(writeFileMock).toHaveBeenCalledTimes(1);
  });

  // A retry that collides again would mean the directory vanished between the
  // two attempts, which nothing here does; failing loudly beats looping.
  it('does not retry twice', async () => {
    writeFileMock.mockRejectedValueOnce(collision).mockRejectedValueOnce(collision);

    await expect(writeTextFile('plainshelf-cache/scopes/s/x.json', '{}')).rejects.toThrow(
      'Current directory does already exist.'
    );
    expect(writeFileMock).toHaveBeenCalledTimes(2);
  });
});
