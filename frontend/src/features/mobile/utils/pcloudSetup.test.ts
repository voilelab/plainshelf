import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  checkPCloudShelf: vi.fn(),
  listPCloudFolder: vi.fn()
}));

vi.mock('@/providers/localShelf', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/providers/localShelf')>()),
  LocalShelf: {
    checkPCloudShelf: mocks.checkPCloudShelf,
    listPCloudFolder: mocks.listPCloudFolder
  }
}));

const { checkPCloudShelf, pcloudFolderClient } = await import('./pcloudSetup');

beforeEach(() => {
  mocks.checkPCloudShelf.mockReset();
  mocks.listPCloudFolder.mockReset();
});

describe('checkPCloudShelf', () => {
  it('passes the form token to Go and returns the count', async () => {
    mocks.checkPCloudShelf.mockResolvedValue({ isShelf: true, books: 3 });

    await expect(checkPCloudShelf('eapi.pcloud.com', 'tok', '/Shelf')).resolves.toBe(3);
    expect(mocks.checkPCloudShelf).toHaveBeenCalledWith({
      host: 'eapi.pcloud.com',
      token: 'tok',
      folder: '/Shelf'
    });
  });

  it('tells an empty shelf from a folder that is not one', async () => {
    mocks.checkPCloudShelf.mockResolvedValueOnce({ isShelf: true, books: 0 });
    mocks.checkPCloudShelf.mockResolvedValueOnce({ isShelf: false, books: 0 });

    await expect(checkPCloudShelf('api.pcloud.com', 'tok', '/Empty')).resolves.toBe(0);
    await expect(checkPCloudShelf('api.pcloud.com', 'tok', '/Photos')).resolves.toBeNull();
  });
});

describe('pcloudFolderClient', () => {
  it('lists a level through Go', async () => {
    mocks.listPCloudFolder.mockResolvedValue({
      folders: [{ name: 'books', folderid: 7 }],
      isShelf: true
    });

    const listing = await pcloudFolderClient('api.pcloud.com', 'tok').listFolder(42);

    expect(listing).toEqual({ folders: [{ name: 'books', folderid: 7 }], isShelf: true });
    expect(mocks.listPCloudFolder).toHaveBeenCalledWith({
      host: 'api.pcloud.com',
      token: 'tok',
      folderId: 42
    });
  });
});
