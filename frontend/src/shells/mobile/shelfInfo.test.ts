import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getActiveShelfEntryMock } = vi.hoisted(() => ({
  getActiveShelfEntryMock: vi.fn()
}));

vi.mock('@/providers/mobileConfig', async () => {
  const actual = await vi.importActual<typeof import('@/providers/mobileConfig')>(
    '@/providers/mobileConfig'
  );
  return {
    ...actual,
    getActiveShelfEntry: getActiveShelfEntryMock
  };
});

const { activeMobileShelfInfo } = await import('./shelfInfo');
type ShelfEntry = import('@/providers/mobileConfig').ShelfEntry;

function serverEntry(overrides: Record<string, unknown> = {}): ShelfEntry {
  return {
    id: 'entry-1',
    type: 'server',
    name: '',
    serverUrl: 'http://host',
    shelfId: 'main',
    ...overrides
  } as ShelfEntry;
}

function pcloudEntry(overrides: Record<string, unknown> = {}): ShelfEntry {
  return {
    id: 'entry-2',
    type: 'pcloud',
    name: '',
    pcloudClientId: '',
    pcloudHost: 'api.pcloud.com',
    pcloudShelfRoot: '/PlainShelf/default-shelf',
    ...overrides
  } as ShelfEntry;
}

beforeEach(() => {
  getActiveShelfEntryMock.mockReset().mockReturnValue(null);
});

describe('activeMobileShelfInfo', () => {
  it('names a pCloud shelf after the last segment of the folder path', () => {
    getActiveShelfEntryMock.mockReturnValue(pcloudEntry());

    // The id is the full path, matching the active shelf id mobileConfig sets,
    // so the device-local cache scope agrees with the picker.
    // readOnly stays false even here: a pCloud entry cannot be written, but
    // that is the provider's own missing write surface rather than a shelf
    // setting, and useWriteAccess already reports it as the platform reason.
    expect(activeMobileShelfInfo()).toEqual({
      id: '/PlainShelf/default-shelf',
      name: 'default-shelf',
      readOnly: false
    });
  });

  it('tolerates a trailing slash', () => {
    getActiveShelfEntryMock.mockReturnValue(pcloudEntry({ pcloudShelfRoot: '/shelf/' }));

    expect(activeMobileShelfInfo()?.name).toBe('shelf');
  });

  it('uses the entry name when the user gave it one', () => {
    getActiveShelfEntryMock.mockReturnValue(serverEntry({ name: 'Living room' }));

    expect(activeMobileShelfInfo()).toEqual({ id: 'main', name: 'Living room', readOnly: false });
  });

  it('returns null off the mobile shell and for an entry with no shelf yet', () => {
    expect(activeMobileShelfInfo()).toBeNull();

    getActiveShelfEntryMock.mockReturnValue(serverEntry({ shelfId: '' }));
    expect(activeMobileShelfInfo()).toBeNull();
  });
});
