import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getBookshelfProviderMock } = vi.hoisted(() => ({
  getBookshelfProviderMock: vi.fn()
}));

vi.mock('@/providers', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/providers')>()),
  getBookshelfProvider: getBookshelfProviderMock
}));

const { useServerMode } = await import('./useServerMode');
const { useShelvesStore } = await import('./useShelvesStore');
const { ServerBookshelfProvider } = await import('@/providers/serverBookshelfProvider');
const { WailsBookshelfProvider } = await import('@/providers/wailsBookshelfProvider');
const { MobileBookshelfProvider } = await import('@/providers/mobileBookshelfProvider');
const { InMemoryMobileBookCache } = await import('@/providers/mobileBookCache');
const { EmbeddedPCloudReader } = await import('@/providers/embeddedPCloud');
const { useWriteAccess } = await import('./useWriteAccess');

const { readOnly } = useServerMode();
const { shelves, selectedShelfID } = useShelvesStore();

function browseShelf(id: 'writable' | 'archive'): void {
  shelves.value = [
    { id: 'writable', name: 'Writable', readOnly: false },
    { id: 'archive', name: 'Archive', readOnly: true }
  ];
  selectedShelfID.value = id;
}

function connectReadingClient(): void {
  getBookshelfProviderMock.mockReturnValue(
    new MobileBookshelfProvider(new ServerBookshelfProvider(), new InMemoryMobileBookCache())
  );
}

function connectWritableClient(): void {
  getBookshelfProviderMock.mockReturnValue(new ServerBookshelfProvider());
}

describe('useWriteAccess', () => {
  beforeEach(() => {
    getBookshelfProviderMock.mockReset();
    readOnly.value = false;
    shelves.value = [];
    selectedShelfID.value = '';
  });

  it('enables writes on a writable server with a writable client', () => {
    connectWritableClient();
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(true);
    expect(writeDisabledReason.value).toBeNull();
  });

  it('disables writes on a reading client even when the server is writable', () => {
    connectReadingClient();
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('platform');
  });

  it('disables writes when the server is read-only on a writable client', () => {
    connectWritableClient();
    readOnly.value = true;
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('server-read-only');
  });

  it('reports the platform reason first when both apply', () => {
    connectReadingClient();
    readOnly.value = true;
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('platform');
  });

  it('disables writes on a read-only shelf of a writable server', () => {
    connectWritableClient();
    browseShelf('archive');
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('shelf-read-only');
  });

  it('keeps writes on the writable shelf beside a read-only one', () => {
    connectWritableClient();
    browseShelf('writable');
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(true);
    expect(writeDisabledReason.value).toBeNull();
  });

  it('follows a shelf switch without re-creating the composable', () => {
    connectWritableClient();
    browseShelf('writable');
    const { writesEnabled } = useWriteAccess();
    expect(writesEnabled.value).toBe(true);

    selectedShelfID.value = 'archive';
    expect(writesEnabled.value).toBe(false);
  });

  it('names the server ahead of the shelf when both are read-only', () => {
    connectWritableClient();
    readOnly.value = true;
    browseShelf('archive');
    const { writeDisabledReason } = useWriteAccess();

    expect(writeDisabledReason.value).toBe('server-read-only');
  });

  it('reports the platform reason ahead of the shelf', () => {
    connectReadingClient();
    browseShelf('archive');
    const { writeDisabledReason } = useWriteAccess();

    expect(writeDisabledReason.value).toBe('platform');
  });

  it('names the shelf in the refusal message only when the shelf is the reason', () => {
    connectWritableClient();
    browseShelf('archive');
    const { writeDisabledMessageKey } = useWriteAccess();
    expect(writeDisabledMessageKey.value).toBe('layout.readOnly.shelfWriteDisabled');

    readOnly.value = true;
    expect(writeDisabledMessageKey.value).toBe('layout.readOnly.writeDisabled');
  });

  it('tracks server mode changes without re-creating the composable', () => {
    connectWritableClient();
    const { writesEnabled } = useWriteAccess();
    expect(writesEnabled.value).toBe(true);

    readOnly.value = true;
    expect(writesEnabled.value).toBe(false);
  });

});

describe('outgoingCopyEnabled', () => {
  beforeEach(() => {
    readOnly.value = false;
    shelves.value = [];
    selectedShelfID.value = '';
  });

  it('survives a read-only shelf that has withdrawn every other write', () => {
    connectWritableClient();
    browseShelf('archive');
    const { writesEnabled, outgoingCopyEnabled } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(outgoingCopyEnabled.value).toBe(true);
  });

  it('goes with a read-only server, which refuses the target too', () => {
    connectWritableClient();
    readOnly.value = true;
    const { outgoingCopyEnabled } = useWriteAccess();

    expect(outgoingCopyEnabled.value).toBe(false);
  });

  it('goes on a client with no write surface at all', () => {
    connectReadingClient();
    const { outgoingCopyEnabled } = useWriteAccess();

    expect(outgoingCopyEnabled.value).toBe(false);
  });
});

describe('platform capabilities', () => {
  beforeEach(() => {
    readOnly.value = false;
    shelves.value = [];
    selectedShelfID.value = '';
  });

  it('offers every capability on a writable client', () => {
    connectWritableClient();
    const { libraryEditingAvailable, serverSettingsEditable, serverAdminAvailable } =
      useWriteAccess();

    expect(libraryEditingAvailable.value).toBe(true);
    expect(serverSettingsEditable.value).toBe(true);
    expect(serverAdminAvailable.value).toBe(true);
  });

  it('withdraws every capability on a reading client', () => {
    connectReadingClient();
    const { libraryEditingAvailable, serverSettingsEditable, serverAdminAvailable } =
      useWriteAccess();

    expect(libraryEditingAvailable.value).toBe(false);
    expect(serverSettingsEditable.value).toBe(false);
    expect(serverAdminAvailable.value).toBe(false);
  });

  it('keeps every capability on a read-only shelf', () => {
    connectWritableClient();
    browseShelf('archive');
    const { writesEnabled, libraryEditingAvailable, serverSettingsEditable, serverAdminAvailable } =
      useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(libraryEditingAvailable.value).toBe(true);
    expect(serverSettingsEditable.value).toBe(true);
    expect(serverAdminAvailable.value).toBe(true);
  });

  it('keeps every capability on a read-only server', () => {
    connectWritableClient();
    readOnly.value = true;
    const { writesEnabled, libraryEditingAvailable, serverSettingsEditable, serverAdminAvailable } =
      useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(libraryEditingAvailable.value).toBe(true);
    expect(serverSettingsEditable.value).toBe(true);
    expect(serverAdminAvailable.value).toBe(true);
  });
});

describe('the provider write surface', () => {
  it('decides whether library editing is available at all', () => {
    connectReadingClient();
    expect(useWriteAccess().libraryEditingAvailable.value).toBe(false);

    connectWritableClient();
    expect(useWriteAccess().libraryEditingAvailable.value).toBe(true);
  });

  it('keeps the desktop shell writable through inheritance', () => {
    getBookshelfProviderMock.mockReturnValue(new WailsBookshelfProvider());

    expect(useWriteAccess().libraryEditingAvailable.value).toBe(true);
  });

  it('treats a pCloud-backed client as read-only', () => {
    getBookshelfProviderMock.mockReturnValue(
      new MobileBookshelfProvider(new EmbeddedPCloudReader(), new InMemoryMobileBookCache())
    );

    expect(useWriteAccess().libraryEditingAvailable.value).toBe(false);
  });
});
