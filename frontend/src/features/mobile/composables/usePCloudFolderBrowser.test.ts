import { describe, expect, it, vi } from 'vitest';

import type { PCloudItem } from '@/api/pcloud/types';
import {
  usePCloudFolderBrowser,
  type PCloudFolderClient,
  type PCloudFolderRef
} from './usePCloudFolderBrowser';

function folder(name: string, folderid: number, contents: PCloudItem[] = []): PCloudItem {
  return { name, isfolder: true, folderid, contents };
}

function file(name: string): PCloudItem {
  return { name, isfolder: false, fileid: 1 };
}

/**
 * A client backed by a folder-id keyed tree, standing in for the network the
 * same way the pCloud client tests stand in for fetch.
 */
function fakeClient(tree: Record<number, PCloudItem[]>) {
  const listFolder = vi.fn(async (folderid: number) => {
    const contents = tree[folderid];
    if (!contents) {
      throw new Error(`no folder ${folderid}`);
    }
    return folder(String(folderid), folderid, contents);
  });

  const resolveFolderTrail = vi.fn(async (path: string) => {
    const trail: PCloudFolderRef[] = [];
    let folderid = 0;
    for (const segment of path.split('/').filter(Boolean)) {
      const match = (tree[folderid] ?? []).find((item) => item.isfolder && item.name === segment);
      if (!match || match.folderid === undefined) {
        throw new Error(`pCloud has no folder named "${segment}".`);
      }
      folderid = match.folderid;
      trail.push({ name: segment, folderid });
    }
    return trail;
  });

  return { listFolder, resolveFolderTrail } as unknown as PCloudFolderClient & {
    listFolder: typeof listFolder;
    resolveFolderTrail: typeof resolveFolderTrail;
  };
}

// /PlainShelf/my-shelf is a shelf; /Photos is not.
const TREE: Record<number, PCloudItem[]> = {
  0: [folder('PlainShelf', 10), folder('Photos', 20), file('notes.txt')],
  10: [folder('my-shelf', 11), folder('archive', 12)],
  11: [folder('books', 13), file('README.md')],
  12: [],
  13: [folder('Fiction', 14), folder('Dune.bookpkg', 15)],
  14: [],
  15: [],
  20: []
};

describe('usePCloudFolderBrowser', () => {
  it('starts at the account root and lists its folders in name order', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('');

    expect(browser.currentPath.value).toBe('/');
    expect(browser.trail.value).toEqual([]);
    expect(browser.folders.value.map((item) => item.name)).toEqual(['Photos', 'PlainShelf']);
    expect(browser.loading.value).toBe(false);
    expect(browser.error.value).toBe('');
  });

  it('opens directly at a path that resolves', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/PlainShelf/my-shelf');

    expect(browser.trail.value).toEqual([
      { name: 'PlainShelf', folderid: 10 },
      { name: 'my-shelf', folderid: 11 }
    ]);
    expect(browser.currentPath.value).toBe('/PlainShelf/my-shelf');
    expect(browser.isShelf.value).toBe(true);
  });

  // A saved path can name a folder that was renamed, or one belonging to the
  // account authorized before this one.
  it('falls back to the root and keeps the reason when a path does not resolve', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/PlainShelf/gone');

    expect(browser.currentPath.value).toBe('/');
    expect(browser.folders.value.map((item) => item.name)).toEqual(['Photos', 'PlainShelf']);
    expect(browser.error.value).toMatch(/"gone"/);
  });

  it('walks down and back up, and reports which level is a shelf', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/');
    await browser.enter({ name: 'PlainShelf', folderid: 10 });
    expect(browser.isShelf.value).toBe(false);

    await browser.enter({ name: 'my-shelf', folderid: 11 });
    expect(browser.currentPath.value).toBe('/PlainShelf/my-shelf');
    expect(browser.isShelf.value).toBe(true);

    await browser.goTo(0);
    expect(browser.currentPath.value).toBe('/PlainShelf');
    expect(browser.isShelf.value).toBe(false);

    await browser.goTo(-1);
    expect(browser.currentPath.value).toBe('/');
  });

  it('lists only folders, and never a book package', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/PlainShelf/my-shelf/books');

    expect(browser.folders.value.map((item) => item.name)).toEqual(['Fiction']);
  });

  it('answers a level it has already listed from memory', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/');
    await browser.enter({ name: 'PlainShelf', folderid: 10 });
    await browser.goTo(-1);
    await browser.enter({ name: 'PlainShelf', folderid: 10 });

    expect(client.listFolder).toHaveBeenCalledTimes(2);
  });

  it('surfaces a failed listing and retries it on demand', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/');
    client.listFolder.mockRejectedValueOnce(new Error('pCloud listfolder timed out.'));
    await browser.enter({ name: 'Photos', folderid: 20 });

    expect(browser.error.value).toBe('pCloud listfolder timed out.');
    expect(browser.loading.value).toBe(false);

    await browser.retry();

    expect(browser.error.value).toBe('');
    expect(browser.folders.value).toEqual([]);
  });

  // Two taps in a row must not let the first, slower reply repaint the level
  // the user has already moved past.
  it('drops the reply to a listing the user navigated away from', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);
    await browser.open('/');

    let releaseSlow = (): void => {};
    client.listFolder.mockImplementationOnce(
      async () =>
        await new Promise<PCloudItem>((resolve) => {
          releaseSlow = () => resolve(folder('PlainShelf', 10, TREE[10]));
        })
    );

    const slow = browser.enter({ name: 'PlainShelf', folderid: 10 });
    await browser.goTo(-1);
    releaseSlow();
    await slow;

    expect(browser.currentPath.value).toBe('/');
    expect(browser.folders.value.map((item) => item.name)).toEqual(['Photos', 'PlainShelf']);
    expect(browser.loading.value).toBe(false);
  });

  it('drops a reply that arrives after the picker was closed', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);
    await browser.open('/');

    let releaseSlow = (): void => {};
    client.listFolder.mockImplementationOnce(
      async () =>
        await new Promise<PCloudItem>((resolve) => {
          releaseSlow = () => resolve(folder('PlainShelf', 10, TREE[10]));
        })
    );

    const slow = browser.enter({ name: 'PlainShelf', folderid: 10 });
    browser.close();
    releaseSlow();
    await slow;

    expect(browser.folders.value).toEqual([]);
    expect(browser.loading.value).toBe(false);
    expect(browser.error.value).toBe('');
  });

  it('re-reads the tree when it is opened again', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/');
    await browser.open('/');

    expect(client.listFolder).toHaveBeenCalledTimes(2);
  });
});
