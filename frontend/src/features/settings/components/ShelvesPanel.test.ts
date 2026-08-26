// @vitest-environment jsdom
import { createApp, defineComponent, h, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The panel's shelf-management surface is desktop-only: the add-shelf button,
// the per-row "Open folder" action and the shelf-ID preview appear on the
// desktop shell and are absent on the server/web build, which instead says the
// shelves are server-managed. These flags drive that split.
const state = vi.hoisted(() => ({ desktop: false, mobile: false }));

vi.mock('@/providers', () => ({
  isWailsRuntime: () => state.desktop,
  isMobileRuntime: () => state.mobile
}));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref: r } = await import('vue');
  return { useWriteAccess: () => ({ serverSettingsEditable: r(true) }) };
});

vi.mock('@/api/shelves', () => ({ exportShelfBookCache: vi.fn() }));

// The two modals render their default slot so the add-shelf form (and its
// shelf-ID preview) is inspectable without standing up the real dialogs. Each
// factory is self-contained — it references only the hoisted `h` import — since
// vi.mock runs above any top-level binding it would otherwise close over.
vi.mock('@/components/ConfirmModal.vue', () => ({
  default: defineComponent({
    setup: (_props, { slots }) => () => h('div', { class: 'modal-stub' }, slots.default?.())
  })
}));
vi.mock('@/components/DeleteModal.vue', () => ({
  default: defineComponent({
    setup: (_props, { slots }) => () => h('div', { class: 'modal-stub' }, slots.default?.())
  })
}));

// Controlled management state so the test decides what the panel renders.
const mgmt = vi.hoisted(() => ({ value: null as Record<string, unknown> | null }));
vi.mock('@/features/settings/composables/useShelfManagement', () => ({
  useShelfManagement: () => mgmt.value
}));

import ShelvesPanel from './ShelvesPanel.vue';
import { setLocale } from '@/i18n';

function buildManagement(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    shelves: ref([{ id: 'my-books', name: 'My Books' }]),
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
    newShelfDirectory: ref(''),
    newShelfScanInterval: ref(''),
    newShelfIDPreview: ref(''),
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

function mount() {
  const host = document.createElement('div');
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
});

describe('ShelvesPanel', () => {
  it('shows shelf-management controls on the desktop shell', () => {
    state.desktop = true;
    const { host, app } = mount();

    const actionCell = host.querySelector('.shelf-action-cell');
    expect(actionCell).not.toBeNull();
    expect(actionCell?.textContent).toContain('Open folder');
    // The `.shelf-add-toggle` class is shared with the book-cache export button,
    // so match the add-shelf toggle by its label instead.
    expect(hasButton(host, 'Add shelf')).toBe(true);
    // The server-managed notice is only for shells that cannot manage shelves.
    expect(host.textContent).not.toContain('managed by the server');

    app.unmount();
  });

  it('hides the desktop controls and explains server management on the web build', () => {
    state.desktop = false;
    const { host, app } = mount();

    expect(host.querySelector('.shelf-action-cell')).toBeNull();
    expect(hasButton(host, 'Add shelf')).toBe(false);
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
});
