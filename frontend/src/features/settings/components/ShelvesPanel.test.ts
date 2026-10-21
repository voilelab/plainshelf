// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref, watch } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ desktop: false, mobile: false }));

vi.mock('@/providers', () => ({
  isWailsRuntime: () => state.desktop,
  isMobileRuntime: () => state.mobile
}));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref: r } = await import('vue');
  return { useWriteAccess: () => ({ serverSettingsEditable: r(true) }) };
});

const shelvesApi = vi.hoisted(() => ({ exportShelfBookCache: vi.fn() }));
vi.mock('@/api/shelves', () => shelvesApi);

vi.mock('@/components/ConfirmModal.vue', () => ({
  default: defineComponent({
    props: {
      open: { type: Boolean, default: false },
      initialFocus: { type: String, default: undefined }
    },
    setup: (props, { slots }) => {
      const root = ref<HTMLElement | null>(null);
      watch(
        () => props.open,
        async (open) => {
          if (!open || !props.initialFocus) return;
          await nextTick();
          root.value?.querySelector<HTMLElement>(props.initialFocus)?.focus();
        },
        { immediate: true }
      );
      return () =>
        h('div', { class: 'modal-stub', ref: root }, props.open ? slots.default?.() : undefined);
    }
  })
}));
vi.mock('@/components/DeleteModal.vue', () => ({
  default: defineComponent({
    setup: (_props, { slots }) => () => h('div', { class: 'modal-stub' }, slots.default?.())
  })
}));

const mgmt = vi.hoisted(() => ({ value: null as Record<string, unknown> | null }));
vi.mock('@/features/settings/composables/useShelfManagement', () => ({
  useShelfManagement: () => mgmt.value
}));

import ShelvesPanel from './ShelvesPanel.vue';
import { setLocale } from '@/i18n';

function buildManagement(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    qrText: ref(''),
    qrShelves: ref([]),
    qrResults: ref([]),
    previewQR: vi.fn(),
    qrStatus: (status: string) => status,
    shelves: ref([{ id: 'my-books', name: 'My Books' }]),
    retryShelf: vi.fn(),
    retryingShelfIDs: ref(new Set<string>()),
    shelvesLoading: ref(false),
    shelvesError: ref(''),
    ensureShelvesLoaded: vi.fn(),
    shelfOpError: ref(''),
    removingShelfIDs: ref(new Set<string>()),
    pendingRemoveShelf: ref(null),
    shelfRemoveModalError: ref(''),
    requestRemoveShelf: vi.fn(),
    cancelRemoveShelf: vi.fn(),
    confirmRemoveShelf: vi.fn(),
    showAddShelfModal: ref(false),
    newShelfName: ref(''),
    newShelfLocationMode: ref('new'),
    newShelfDirectory: ref(''),
    newShelfDirectoryError: ref(''),
    newShelfReadOnly: ref(false),
    newShelfIDPreview: ref(''),
    newShelfEffectiveDirectory: ref(''),
    addingShelf: ref(false),
    addShelfError: ref(''),
    canSubmitAddShelf: ref(false),
    openAddShelfModal: vi.fn(),
    closeAddShelfModal: vi.fn(),
    onBrowseShelfDirectory: vi.fn(),
    onSubmitAddShelf: vi.fn(),
    openShelfFolder: vi.fn(),
    pendingModifyShelf: ref(null),
    showModifyShelfModal: ref(false),
    modifyShelfName: ref(''),
    modifyShelfScanInterval: ref(''),
    modifyShelfBookCheckInterval: ref(''),
    modifyShelfReadOnly: ref(false),
    modifyShelfPath: ref(''),
    modifyingShelf: ref(false),
    modifyShelfError: ref(''),
    canSubmitModifyShelf: ref(false),
    requestModifyShelf: vi.fn(),
    closeModifyShelfModal: vi.fn(),
    onSubmitModifyShelf: vi.fn(),
    ...overrides
  };
}

function hasButton(host: HTMLElement, label: string): boolean {
  return Array.from(host.querySelectorAll('button')).some((b) => b.textContent?.trim() === label);
}

const hosts: HTMLElement[] = [];

function mount() {
  const host = document.createElement('div');
  document.body.appendChild(host);
  hosts.push(host);
  const app = createApp(ShelvesPanel);
  app.component(
    'RouterLink',
    defineComponent({
      props: { to: { type: [String, Object], default: '' } },
      setup: (_p, { slots }) => () => h('a', {}, slots.default?.())
    })
  );
  app.mount(host);
  return { host, app };
}

beforeEach(() => {
  setLocale('en');
  state.desktop = false;
  state.mobile = false;
  mgmt.value = buildManagement();
});

afterEach(() => {
  vi.clearAllMocks();
  hosts.splice(0).forEach((host) => host.remove());
});

describe('ShelvesPanel', () => {
  it('shows shelf-management controls on the desktop shell', () => {
    state.desktop = true;
    const { host, app } = mount();

    const actionCell = host.querySelector('.shelf-action-cell');
    expect(actionCell).not.toBeNull();
    expect(actionCell?.textContent).toContain('Open folder');
    expect(hasButton(host, 'Create shelf')).toBe(true);
    expect(host.textContent).not.toContain('managed by the server');

    app.unmount();
  });

  it('hides the desktop controls and explains server management on the web build', () => {
    state.desktop = false;
    const { host, app } = mount();

    expect(host.querySelector('.shelf-action-cell')).toBeNull();
    expect(hasButton(host, 'Create shelf')).toBe(false);
    expect(host.textContent).toContain('managed by the server');

    app.unmount();
  });

  it('previews the shelf ID in the add-shelf form', () => {
    state.desktop = true;
    mgmt.value = buildManagement({
      showAddShelfModal: ref(true),
      newShelfName: ref('小說'),
      newShelfIDPreview: ref('shelf')
    });
    const { host, app } = mount();

    const preview = host.querySelector('.shelf-id-preview');
    expect(preview).not.toBeNull();
    expect(preview?.textContent).toContain('shelf');

    app.unmount();
  });

  it('asks only for a name on the new-folder branch, and names the folder it will create', () => {
    state.desktop = true;
    mgmt.value = buildManagement({
      showAddShelfModal: ref(true),
      newShelfName: ref('Novels'),
      newShelfIDPreview: ref('novels'),
      newShelfEffectiveDirectory: ref('/config/PlainShelf/shelves/novels')
    });
    const { host, app } = mount();

    expect(host.querySelector('[data-testid="shelf-default-path"]')?.textContent).toContain(
      '/config/PlainShelf/shelves/novels'
    );
    expect(host.querySelector('.shelf-id-preview')?.textContent).toContain('novels');
    expect(host.querySelector('[data-testid="shelf-directory-input"]')).toBeNull();
    expect(host.querySelector('[data-testid="shelf-read-only"]')).toBeNull();
    expect(host.querySelector('[data-testid="scan-interval-mode"]')).toBeNull();
    expect(host.querySelector('details.shelf-advanced')).toBeNull();

    app.unmount();
  });

  it('reveals the path box and the read-only toggle on the existing-folder branch', () => {
    state.desktop = true;
    mgmt.value = buildManagement({
      showAddShelfModal: ref(true),
      newShelfLocationMode: ref('existing'),
      newShelfName: ref('Archive'),
      newShelfIDPreview: ref('archive'),
      newShelfEffectiveDirectory: ref('books/archive'),
      newShelfDirectoryError: ref('Enter a full path, starting from the root of the drive.')
    });
    const { host, app } = mount();

    expect(host.querySelector('[data-testid="shelf-directory-input"]')).not.toBeNull();
    expect(host.querySelector('[data-testid="shelf-read-only"]')).not.toBeNull();
    expect(host.querySelector('[data-testid="shelf-directory-error"]')?.textContent).toContain(
      'full path'
    );
    expect(host.querySelector('[data-testid="shelf-default-path"]')).toBeNull();
    expect(host.querySelector('.shelf-id-preview')?.textContent).toContain('archive');

    app.unmount();
  });

  it('switches the location branch when the other option is picked', async () => {
    state.desktop = true;
    const newShelfLocationMode = ref('new');
    mgmt.value = buildManagement({ showAddShelfModal: ref(true), newShelfLocationMode });
    const { host, app } = mount();

    host.querySelector<HTMLElement>('[data-testid="shelf-location-existing"]')!.click();
    await Promise.resolve();
    expect(newShelfLocationMode.value).toBe('existing');

    app.unmount();
  });

  it('loads a stored interval into the modify-shelf controls', () => {
    state.desktop = true;
    mgmt.value = buildManagement({
      showModifyShelfModal: ref(true),
      pendingModifyShelf: ref({ id: 'my-books', name: 'My Books' }),
      modifyShelfScanInterval: ref('1h30m')
    });
    const { host, app } = mount();

    expect(host.querySelector<HTMLSelectElement>('[data-testid="scan-interval-mode"]')?.value).toBe(
      'interval'
    );
    expect(host.querySelector<HTMLInputElement>('[data-testid="scan-interval-amount"]')?.value).toBe(
      '90'
    );
    expect(host.querySelector<HTMLSelectElement>('[data-testid="scan-interval-unit"]')?.value).toBe(
      'm'
    );

    app.unmount();
  });

  it('edits book_check_interval inside a collapsed advanced section of the modify-shelf form', async () => {
    state.desktop = true;
    const modifyShelfBookCheckInterval = ref('');
    mgmt.value = buildManagement({
      showModifyShelfModal: ref(true),
      pendingModifyShelf: ref({ id: 'my-books', name: 'My Books' }),
      modifyShelfBookCheckInterval
    });
    const { host, app } = mount();

    const advanced = host.querySelector('details.shelf-advanced');
    expect(advanced).not.toBeNull();
    const mode = advanced!.querySelector<HTMLSelectElement>(
      '[data-testid="book-check-interval-mode"]'
    );
    expect(mode).not.toBeNull();

    mode!.value = 'always';
    mode!.dispatchEvent(new Event('change'));
    await Promise.resolve();
    expect(modifyShelfBookCheckInterval.value).toBe('0s');

    app.unmount();
  });

  it('offers the read-only toggle when creating a shelf, with its knock-on effects spelled out', async () => {
    state.desktop = true;
    const newShelfReadOnly = ref(false);
    mgmt.value = buildManagement({
      showAddShelfModal: ref(true),
      newShelfLocationMode: ref('existing'),
      newShelfReadOnly
    });
    const { host, app } = mount();

    const toggle = host.querySelector<HTMLInputElement>('[data-testid="shelf-read-only"]');
    expect(toggle).not.toBeNull();
    expect(host.textContent).toContain('File locking is turned off');
    expect(host.textContent).toContain('exported book cache is not written');
    expect(host.textContent).toContain('The directory is never created');

    toggle!.checked = true;
    toggle!.dispatchEvent(new Event('change'));
    await Promise.resolve();
    expect(newShelfReadOnly.value).toBe(true);

    app.unmount();
  });

  it('shows the read-only toggle as on and operable for an already read-only shelf', async () => {
    state.desktop = true;
    const modifyShelfReadOnly = ref(true);
    mgmt.value = buildManagement({
      showModifyShelfModal: ref(true),
      pendingModifyShelf: ref({ id: 'archive', name: 'Archive' }),
      modifyShelfReadOnly
    });
    const { host, app } = mount();

    const toggle = host.querySelector<HTMLInputElement>('[data-testid="shelf-read-only"]');
    expect(toggle).not.toBeNull();
    expect(toggle!.checked).toBe(true);
    expect(toggle!.disabled).toBe(false);

    toggle!.checked = false;
    toggle!.dispatchEvent(new Event('change'));
    await Promise.resolve();
    expect(modifyShelfReadOnly.value).toBe(false);

    app.unmount();
  });

  it('opens the create-shelf dialog with the caret in the name field', async () => {
    state.desktop = true;
    mgmt.value = buildManagement({ showAddShelfModal: ref(true) });
    const { host, app } = mount();
    await nextTick();
    await nextTick();

    const nameInput = host.querySelector<HTMLInputElement>('[data-testid="shelf-name-input"]');
    expect(nameInput).not.toBeNull();
    expect(document.activeElement).toBe(nameInput);

    app.unmount();
  });

  it('labels every field of the create-shelf form', () => {
    state.desktop = true;
    mgmt.value = buildManagement({
      showAddShelfModal: ref(true),
      newShelfLocationMode: ref('existing')
    });
    const { host, app } = mount();

    for (const testid of ['shelf-name-input', 'shelf-directory-input']) {
      const input = host.querySelector<HTMLInputElement>(`[data-testid="${testid}"]`)!;
      expect(input.id).not.toBe('');
      const label = host.querySelector(`label[for="${input.id}"]`);
      expect(label?.textContent?.trim()).toBeTruthy();
    }

    app.unmount();
  });

  it('promotes creating a shelf to the primary action while no shelf exists', () => {
    state.desktop = true;
    mgmt.value = buildManagement({ shelves: ref([]) });
    const { host, app } = mount();

    const primary = host.querySelector<HTMLButtonElement>('.shelf-empty-add');
    expect(primary).not.toBeNull();
    expect(primary!.textContent?.trim()).toBe('Create shelf');
    expect(primary!.classList.contains('primary')).toBe(true);
    expect(
      Array.from(host.querySelectorAll('.shelf-add-toggle')).some(
        (b) => b.textContent?.trim() === 'Create shelf'
      )
    ).toBe(false);

    app.unmount();
  });

  it('keeps the secondary toggle under the table once a shelf exists', () => {
    state.desktop = true;
    const { host, app } = mount();

    expect(host.querySelector('.shelf-empty-add')).toBeNull();
    expect(
      Array.from(host.querySelectorAll('.shelf-add-toggle')).some(
        (b) => b.textContent?.trim() === 'Create shelf'
      )
    ).toBe(true);

    app.unmount();
  });

  it('reveals a shelf folder when its Open folder button is clicked', () => {
    state.desktop = true;
    const openShelfFolder = vi.fn();
    mgmt.value = buildManagement({ openShelfFolder });
    const { host, app } = mount();

    const button = Array.from(host.querySelectorAll<HTMLButtonElement>('.shelf-action-cell button')).find(
      (b) => b.textContent?.includes('Open folder')
    );
    button?.click();
    expect(openShelfFolder).toHaveBeenCalledWith('my-books');

    app.unmount();
  });

  it('rewrites the book cache only for the writable shelves', async () => {
    shelvesApi.exportShelfBookCache.mockReset().mockResolvedValue(0);
    mgmt.value = buildManagement({
      shelves: ref([
        { id: 'my-books', name: 'My Books', readOnly: false },
        { id: 'archive', name: 'Archive', readOnly: true },
        { id: 'drafts', name: 'Drafts', readOnly: false }
      ])
    });
    const { host, app } = mount();

    const exportButton = Array.from(host.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Update now'
    );
    exportButton?.click();
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(shelvesApi.exportShelfBookCache.mock.calls.map(([id]) => id)).toEqual([
      'my-books',
      'drafts'
    ]);

    app.unmount();
  });
});

vi.mock('@/providers/platformCapabilities', () => ({
  getPlatformCapabilities: () => ({ shelves: state.desktop ? { previewLANShareQR: vi.fn() } : undefined })
}));

it('shows source and safe failure state with retry and remove, without local-only actions', async () => {
  state.desktop = true;
  const retryShelf = vi.fn();
  mgmt.value = buildManagement({
    shelves: ref([
      { id: 'local', name: 'Local' },
      {
        id: 'remote',
        name: 'Remote',
        source: 'webdav',
        url: 'https://example.test/shelf',
        open_error: 'safe'
      },
      {
        id: 'ready',
        name: 'Ready',
        source: 'plainshelf',
        url: 'https://example.test/dav'
      }
    ]),
    retryShelf
  });
  const { host, app } = mount();
  const rows = host.querySelectorAll('tbody tr');
  expect(rows[0]?.textContent).toContain('Open folder');
  expect(rows[1]?.textContent).toContain('webdav');
  expect(rows[1]?.textContent).toContain('https://example.test/shelf');
  expect(rows[1]?.textContent).toContain('Could not open this virtual shelf');
  expect(rows[1]?.textContent).not.toContain('Open folder');
  expect(rows[1]?.textContent).not.toContain('Modify');
  expect(rows[1]?.textContent).toContain('Delete');
  rows[1]?.querySelector<HTMLButtonElement>('button')?.click();
  expect(retryShelf).toHaveBeenCalledWith('remote');
  expect(rows[2]?.textContent).not.toContain('Open folder');
  expect(rows[2]?.textContent).toContain('Modify');
  app.unmount();
});

it('hides the read-only toggle while modifying a virtual shelf', () => {
  mgmt.value = buildManagement({
    showModifyShelfModal: ref(true),
    modifyShelfIsVirtual: ref(true)
  });
  const { host, app } = mount();
  expect(host.querySelector('[data-testid="shelf-read-only"]')).toBeNull();
  expect(host.querySelector('button[role="switch"]')).toBeNull();
  app.unmount();
});



it('renders the QR preview and per-source results in the add dialog', async () => {
  state.desktop = true;
  const previewQR = vi.fn();
  mgmt.value = buildManagement({
    showAddShelfModal: ref(true),
    newShelfLocationMode: ref('qr'),
    qrText: ref('pasted JSON'),
    qrShelves: ref([{ id: 'one', name: 'One', url: 'http://lan/dav/one/' }]),
    qrResults: ref([{ name: 'Two', url: 'http://lan/dav/two/', status: 'SOURCE_UNAVAILABLE' }]),
    previewQR
  });
  const { host } = mount();
  await nextTick();
  expect(host.querySelector('[data-testid="shelf-location-qr"]')).not.toBeNull();
  expect(host.querySelector('[data-testid="shelf-qr-json"]')).not.toBeNull();
  expect(host.querySelector('[data-testid="shelf-qr-preview"]')?.textContent).toContain('One — http://lan/dav/one/');
  expect(host.querySelector('[data-testid="shelf-qr-results"]')?.textContent).toContain('Two — SOURCE_UNAVAILABLE');
  const preview = Array.from(host.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Preview shelves');
  preview!.click();
  expect(previewQR).toHaveBeenCalledOnce();
});
