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
const { ServerBookshelfProvider } = await import('@/providers/serverBookshelfProvider');
const { WailsBookshelfProvider } = await import('@/providers/wailsBookshelfProvider');
const { MobileBookshelfProvider } = await import('@/providers/mobileBookshelfProvider');
const { InMemoryMobileBookCache } = await import('@/providers/mobileBookCache');
const { PCloudBookshelfProvider } = await import('@/providers/pcloudBookshelfProvider');
const { useShelvesStore } = await import('./useShelvesStore');
const { isLibraryEditingSupported, useWriteAccess } = await import('./useWriteAccess');

// readOnly is a module-level ref shared by every useServerMode() caller, so the
// test drives it directly instead of stubbing GET /api/mode.
const { readOnly } = useServerMode();

// The shelf list is module-level for the same reason, so the per-shelf flag is
// driven by loading a list rather than by stubbing GET /api/shelves.
const { shelves, selectedShelfID } = useShelvesStore();

/**
 * Loads one read-only shelf beside one writable shelf and selects the named
 * one, which is the arrangement the per-shelf flag exists to tell apart.
 */
function selectShelfAmongMixedShelves(id: 'writable' | 'read-only'): void {
  shelves.value = [
    { id: 'writable', name: 'Writable', readOnly: false },
    { id: 'read-only', name: 'Read-only', readOnly: true }
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

// Server mode and the shelf list are module-level singletons, so every case
// starts from a writable server with no shelf list rather than from whatever
// the previous one left behind.
beforeEach(() => {
  getBookshelfProviderMock.mockReset();
  readOnly.value = false;
  shelves.value = [];
  selectedShelfID.value = '';
});

describe('useWriteAccess', () => {

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

  // The whole point of the flag: two shelves of one writable server, and only
  // the read-only one loses its write entries.
  it('disables writes only on the read-only shelf of a mixed pair', () => {
    connectWritableClient();
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    selectShelfAmongMixedShelves('writable');
    expect(writesEnabled.value).toBe(true);
    expect(writeDisabledReason.value).toBeNull();

    selectShelfAmongMixedShelves('read-only');
    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('shelf-read-only');
  });

  // A shelf resolved from the device-local fallback has no listing entry, and
  // an unknown shelf is not one this client may call read-only on its own.
  it('treats a shelf missing from the list as writable', () => {
    connectWritableClient();
    selectShelfAmongMixedShelves('read-only');
    selectedShelfID.value = 'not-in-the-list';
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(true);
    expect(writeDisabledReason.value).toBeNull();
  });

  it('reports the server reason first when the shelf is read-only too', () => {
    connectWritableClient();
    readOnly.value = true;
    selectShelfAmongMixedShelves('read-only');
    const { writeDisabledReason } = useWriteAccess();

    expect(writeDisabledReason.value).toBe('server-read-only');
  });

  it('names the shelf in the message a refused write reports', () => {
    connectWritableClient();
    const { writeDisabledMessage } = useWriteAccess();

    selectShelfAmongMixedShelves('read-only');
    const shelfMessage = writeDisabledMessage();

    readOnly.value = true;
    expect(writeDisabledMessage()).not.toBe(shelfMessage);
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

  // A read-only shelf is a narrower statement than a read-only client: Trash,
  // the maintenance lists and the settings tabs stay reachable, they just have
  // nothing to act on for this shelf.
  it('keeps every capability on a read-only shelf', () => {
    connectWritableClient();
    selectShelfAmongMixedShelves('read-only');
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
