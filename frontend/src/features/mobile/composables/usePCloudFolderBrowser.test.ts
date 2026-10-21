import { describe, expect, it, vi } from 'vitest';

import { PCloudError } from '@/api/pcloud/errors';
import { useErrorIncident } from '@/composables/useErrorIncident';
import {
  usePCloudFolderBrowser,
  type PCloudFolderClient,
  type PCloudFolderRef
} from './usePCloudFolderBrowser';

type PCloudFolderListing = Awaited<ReturnType<PCloudFolderClient['listFolder']>>;

function fakeClient(tree: Record<number, PCloudFolderListing>) {
  const listFolder = vi.fn(async (folderid: number) => {
    const listing = tree[folderid];
    if (!listing) {
      throw new Error(`no folder ${folderid}`);
    }
    return listing;
  });

  const resolveFolderTrail = vi.fn(async (path: string) => {
    const trail: PCloudFolderRef[] = [];
    let folderid = 0;
    for (const segment of path.split('/').filter(Boolean)) {
      const match = (tree[folderid]?.folders ?? []).find((item) => item.name === segment);
      if (!match) {
        throw new Error(`pCloud has no folder named "${segment}".`);
      }
      folderid = match.folderid;
      trail.push({ name: segment, folderid });
    }
    return trail;
  });

  return { listFolder, resolveFolderTrail } satisfies PCloudFolderClient;
}

function level(folders: Array<[string, number]>, isShelf = false): PCloudFolderListing {
  return { folders: folders.map(([name, folderid]) => ({ name, folderid })), isShelf };
}

const TREE: Record<number, PCloudFolderListing> = {
  0: level([
    ['PlainShelf', 10],
    ['Photos', 20]
  ]),
  10: level([
    ['my-shelf', 11],
    ['archive', 12]
  ]),
  11: level([['books', 13]], true),
  12: level([]),
  13: level([['Fiction', 14]]),
  14: level([]),
  20: level([])
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

  it('falls back to a browsable root and explains why, when a path does not resolve', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/PlainShelf/gone');

    expect(browser.currentPath.value).toBe('/');
    expect(browser.folders.value.map((item) => item.name)).toEqual(['Photos', 'PlainShelf']);
    expect(browser.notice.value).toMatch(/"gone"/);
    expect(browser.error.value).toBe('');
  });

  it('drops the explanation once the user navigates', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    await browser.open('/PlainShelf/gone');
    await browser.enter({ name: 'PlainShelf', folderid: 10 });

    expect(browser.notice.value).toBe('');
  });

  it('reports a failed fallback listing on its own terms', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);

    client.listFolder.mockRejectedValueOnce(new Error('pCloud listfolder timed out.'));
    await browser.open('/PlainShelf/gone');

    expect(browser.error.value).toBe('pCloud listfolder timed out.');
    expect(browser.notice.value).toMatch(/"gone"/);
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

  it('publishes the reference of a failed listing', async () => {
    const { incident, dismissIncident } = useErrorIncident();
    dismissIncident();

    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);
    await browser.open('/');

    const failure = new PCloudError('pCloud listfolder failed.');
    client.listFolder.mockRejectedValueOnce(failure);
    await browser.enter({ name: 'Photos', folderid: 20 });

    expect(browser.error.value).toBe('pCloud listfolder failed.');
    expect(incident.value).toBe(failure.incident);
    expect(incident.value).toMatch(/^c-/);
  });

  it('publishes nothing for a listing the user navigated away from', async () => {
    const { incident, dismissIncident } = useErrorIncident();
    dismissIncident();

    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);
    await browser.open('/');

    let failSlow = (): void => {};
    client.listFolder.mockImplementationOnce(
      async () =>
        await new Promise<PCloudFolderListing>((_resolve, reject) => {
          failSlow = () => reject(new PCloudError('pCloud listfolder failed.'));
        })
    );

    const slow = browser.enter({ name: 'PlainShelf', folderid: 10 });
    await browser.goTo(-1);
    failSlow();
    await slow;

    expect(incident.value).toBe('');
  });

  it('drops the reply to a listing the user navigated away from', async () => {
    const client = fakeClient(TREE);
    const browser = usePCloudFolderBrowser(() => client);
    await browser.open('/');

    let releaseSlow = (): void => {};
    client.listFolder.mockImplementationOnce(
      async () =>
        await new Promise<PCloudFolderListing>((resolve) => {
          releaseSlow = () => resolve(TREE[10]);
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
        await new Promise<PCloudFolderListing>((resolve) => {
          releaseSlow = () => resolve(TREE[10]);
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
