import { beforeEach, describe, expect, it, vi } from 'vitest';

const { rmdirMock } = vi.hoisted(() => ({ rmdirMock: vi.fn() }));

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: { rmdir: rmdirMock }
}));

const { removeCacheScope, scopeDir, UNSCOPED_DIR_NAME } = await import('./mobileCacheFs');

beforeEach(() => {
  rmdirMock.mockReset().mockResolvedValue(undefined);
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
