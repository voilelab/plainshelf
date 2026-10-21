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
vi.mock('vue-router', () => ({ RouterLink: { render: () => null }, RouterView: { render: () => null } }));

const { default: ReaderLayout } = await import('./ReaderLayout.vue');
const { useWriteAccess } = await import('@/composables/useWriteAccess');
const { useShelvesStore } = await import('@/composables/useShelvesStore');
const { useServerMode } = await import('@/composables/useServerMode');
const { setActiveShelfID } = await import('@/api/client');

const { shelves, loaded, selectedShelfID } = useShelvesStore();
const { readOnly, loaded: modeLoaded } = useServerMode();

let mounted: App | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  mounted = createApp({ setup: () => () => h(ReaderLayout) });
  mounted.mount(host);
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

beforeEach(() => {
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

  it('fetches the server mode so a read-only server disables writes', async () => {
    listShelvesMock.mockResolvedValue([{ id: 'main', name: 'Main', readOnly: false }]);
    getReadOnlyModeMock.mockResolvedValue(true);

    mount();
    await flush();

    expect(useWriteAccess().writesEnabled.value).toBe(false);
    expect(useWriteAccess().writeDisabledReason.value).toBe('server-read-only');
  });
});

describe('ReaderLayout boot gate', () => {
  it('holds the page back until the server mode has answered too', async () => {
    listShelvesMock.mockResolvedValue([{ id: 'main', name: 'Main', readOnly: false }]);
    let releaseMode: (value: boolean) => void = () => {};
    getReadOnlyModeMock.mockReturnValue(
      new Promise<boolean>((resolve) => {
        releaseMode = resolve;
      })
    );

    const host = mount();
    await flush();

    expect(selectedShelfID.value).toBe('main');
    expect(host.querySelector('routerview')).toBeNull();
    expect(host.querySelector('.reader-no-shelf')).not.toBeNull();

    releaseMode(true);
    await flush();

    expect(host.querySelector('routerview')).not.toBeNull();
    expect(useWriteAccess().writesEnabled.value).toBe(false);
  });
});
