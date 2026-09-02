// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, type App } from 'vue';

const { listShelvesMock, getReadOnlyModeMock } = vi.hoisted(() => ({
  listShelvesMock: vi.fn(),
  getReadOnlyModeMock: vi.fn()
}));

vi.mock('@/api/shelves', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/shelves')>()),
  listShelves: listShelvesMock
}));
vi.mock('@/api/mode', () => ({ getReadOnlyMode: getReadOnlyModeMock }));
// The RouterLink in the no-shelf state is the only router dependency here.
vi.mock('vue-router', () => ({ RouterLink: { render: () => null }, RouterView: { render: () => null } }));

const { default: ReaderLayout } = await import('./ReaderLayout.vue');
const { useWriteAccess } = await import('@/composables/useWriteAccess');
const { useShelvesStore } = await import('@/composables/useShelvesStore');
const { useServerMode } = await import('@/composables/useServerMode');
const { setActiveShelfID } = await import('@/api/client');

const { shelves, loaded, selectedShelfID } = useShelvesStore();
const { readOnly, loaded: modeLoaded } = useServerMode();

let mounted: App | null = null;

function mount(): void {
  const host = document.createElement('div');
  document.body.append(host);
  mounted = createApp({ setup: () => () => h(ReaderLayout) });
  mounted.mount(host);
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

beforeEach(() => {
  // The store and useServerMode are module singletons shared across tests.
  shelves.value = [];
  loaded.value = false;
  selectedShelfID.value = '';
  readOnly.value = false;
  modeLoaded.value = false;
  setActiveShelfID('');
  listShelvesMock.mockReset();
  getReadOnlyModeMock.mockReset().mockResolvedValue(false);
});

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
});

// The source editor lives under this layout and asks useWriteAccess whether it
// may write. A layout that kept its shelf list to itself left the shared store
// empty, so opening /books/:id/sources directly offered every edit on a
// read-only shelf until the server refused it with 409.
describe('ReaderLayout shelf state', () => {
  it('populates the shared store so a read-only shelf disables writes', async () => {
    listShelvesMock.mockResolvedValue([{ id: 'archive', name: 'Archive', readOnly: true }]);

    mount();
    await flush();

    expect(selectedShelfID.value).toBe('archive');
    expect(useWriteAccess().writesEnabled.value).toBe(false);
    expect(useWriteAccess().writeDisabledReason.value).toBe('shelf-read-only');
  });

  it('leaves writes enabled on a writable shelf', async () => {
    listShelvesMock.mockResolvedValue([{ id: 'main', name: 'Main', readOnly: false }]);

    mount();
    await flush();

    expect(selectedShelfID.value).toBe('main');
    expect(useWriteAccess().writesEnabled.value).toBe(true);
  });

  // The other input to the same gate: a fresh load here never asked for the
  // server's mode either, so a read-only server was equally invisible.
  it('fetches the server mode so a read-only server disables writes', async () => {
    listShelvesMock.mockResolvedValue([{ id: 'main', name: 'Main', readOnly: false }]);
    getReadOnlyModeMock.mockResolvedValue(true);

    mount();
    await flush();

    expect(useWriteAccess().writesEnabled.value).toBe(false);
    expect(useWriteAccess().writeDisabledReason.value).toBe('server-read-only');
  });
});
