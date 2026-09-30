// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, ref, type App } from 'vue';

const { listFoldersMock } = vi.hoisted(() => ({ listFoldersMock: vi.fn() }));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listFolders: listFoldersMock })
}));
// BaseDialog portals to document.body and traps focus; the modal's own markup is
// what these cases inspect, so it renders as a plain wrapper here.
vi.mock('@/components/BaseDialog.vue', () => ({
  default: { props: ['open', 'title', 'busy'], setup: (_: unknown, { slots }: any) => () => h('div', slots.default?.()) }
}));

const { default: TransferBookModal } = await import('./TransferBookModal.vue');
const { useShelvesStore } = await import('@/composables/useShelvesStore');
const { setLocale } = await import('@/i18n');

const { shelves, selectedShelfID } = useShelvesStore();

let mounted: App | null = null;
const submitted = ref<unknown[]>([]);

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  mounted = createApp({
    setup: () => () =>
      h(TransferBookModal, {
        open: true,
        bookTitle: 'Dune',
        busy: false,
        started: false,
        finished: false,
        status: 'pending',
        percentage: 0,
        onSubmit: (payload: unknown) => submitted.value.push(payload)
      })
  });
  mounted.mount(host);
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function radioValues(host: HTMLElement): string[] {
  return [...host.querySelectorAll<HTMLInputElement>('input[type="radio"]')].map((el) => el.value);
}

beforeEach(() => {
  setLocale('en');
  submitted.value = [];
  listFoldersMock.mockReset().mockResolvedValue([]);
  shelves.value = [
    { id: 'archive', name: 'Archive', readOnly: true },
    { id: 'main', name: 'Main', readOnly: false },
    { id: 'other', name: 'Other', readOnly: false }
  ];
  selectedShelfID.value = 'main';
});

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
});

describe('TransferBookModal destinations', () => {
  // A transfer writes its target, so a read-only shelf can never receive one.
  it('leaves read-only shelves out of the destination list', async () => {
    const host = mount();
    await flush();

    const options = [...host.querySelectorAll('option')]
      .map((el) => (el as HTMLOptionElement).value)
      .filter((value) => value !== '' && value !== '/');
    expect(options).toEqual(['other']);
  });
});

describe('TransferBookModal on a read-only source', () => {
  beforeEach(() => {
    selectedShelfID.value = 'archive';
  });

  // Copying out reads the source and writes the target, so it stays; only the
  // move is refused, because it ends by deleting the original.
  it('offers copy alone, with an explanation in place of the move option', async () => {
    const host = mount();
    await flush();

    expect(radioValues(host)).toEqual(['copy']);
    expect(host.textContent).toContain('can only be copied out of it');
  });

  it('keeps both options on a writable source', async () => {
    selectedShelfID.value = 'main';
    const host = mount();
    await flush();

    expect(radioValues(host)).toEqual(['copy', 'move']);
  });
});

// The modal is a module-level singleton's consumer, not remounted per shelf, so
// a 'move' picked while a writable shelf was selected outlives the switch to a
// read-only one. Hiding the radio is not enough: what is submitted has to
// change with it, or the server answers 409 on a request the UI thinks it
// allowed.
describe('TransferBookModal after a shelf switch', () => {
  it('submits copy for a move picked before the source became read-only', async () => {
    selectedShelfID.value = 'main';
    const host = mount();
    await flush();

    const moveRadio = [...host.querySelectorAll<HTMLInputElement>('input[type="radio"]')].find(
      (el) => el.value === 'move'
    );
    moveRadio!.click();
    await flush();

    selectedShelfID.value = 'archive';
    await flush();
    expect(radioValues(host)).toEqual(['copy']);

    const select = host.querySelector('select') as HTMLSelectElement;
    select.value = 'main';
    select.dispatchEvent(new Event('change'));
    await flush();

    const confirm = [...host.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Transfer'
    );
    confirm?.click();
    await flush();

    expect(submitted.value).toEqual([{ targetShelfId: 'main', targetFolder: '', mode: 'copy' }]);
  });
});
