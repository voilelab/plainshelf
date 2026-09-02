import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getBookshelfProviderMock } = vi.hoisted(() => ({
  getBookshelfProviderMock: vi.fn()
}));

// isWritableProvider is deliberately NOT mocked: it is the predicate under
// test here, so the tests hand it real provider shapes and let it narrow.
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
const { PCloudBookshelfProvider } = await import('@/providers/pcloudBookshelfProvider');
const { isLibraryEditingSupported, useWriteAccess } = await import('./useWriteAccess');

// readOnly is a module-level ref shared by every useServerMode() caller, so the
// test drives it directly instead of stubbing GET /api/mode.
const { readOnly } = useServerMode();
// Same for the shelf list: activeShelfReadOnly is derived from these two refs,
// so seeding them is how a read-only shelf is put in front of the composable.
const { shelves, selectedShelfID } = useShelvesStore();

/**
 * Lists one read-only and one writable shelf and selects the named one, which
 * is the arrangement the feature exists for: the two must not look alike.
 */
function browseShelf(id: 'writable' | 'archive'): void {
  shelves.value = [
    { id: 'writable', name: 'Writable', readOnly: false },
    { id: 'archive', name: 'Archive', readOnly: true }
  ];
  selectedShelfID.value = id;
}

/** Points the composable at a real provider instance, not a stub shape. */
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

  // The same list, the other shelf: this is the acceptance case for the whole
  // change, and it fails if the flag is read off the list rather than the
  // selection.
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

  // A read-only server opens every shelf read-only, so both reasons apply at
  // once. Naming the server is what points the user at the setting that is
  // actually in the way.
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

  // The refusal message must not blame a server the user would then find
  // writable.
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

// These answer "what can a reading client not do", so they must follow the
// client alone. A read-only *server* still shows Trash, the maintenance lists,
// the settings tabs, and the logs — read-only there means the controls render
// disabled, not that the pages disappear.
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

describe('isLibraryEditingSupported', () => {
  it('follows the provider write surface', () => {
    connectReadingClient();
    expect(isLibraryEditingSupported()).toBe(false);

    connectWritableClient();
    expect(isLibraryEditingSupported()).toBe(true);
  });

  // WailsBookshelfProvider inherits its write surface from
  // ServerBookshelfProvider rather than declaring one. If that ever stops
  // being true, the desktop app silently loses every editing affordance —
  // which no mobile test would catch.
  it('keeps the desktop shell writable through inheritance', () => {
    getBookshelfProviderMock.mockReturnValue(new WailsBookshelfProvider());

    expect(isLibraryEditingSupported()).toBe(true);
  });

  // The pCloud backend is a reading client in its own right, independently of
  // which runtime it is reached from.
  it('treats a pCloud-backed client as read-only', () => {
    getBookshelfProviderMock.mockReturnValue(
      new PCloudBookshelfProvider({
        client: {} as ConstructorParameters<typeof PCloudBookshelfProvider>[0]['client'],
        shelfRoot: '/plainshelf'
      })
    );

    expect(isLibraryEditingSupported()).toBe(false);
  });
});
