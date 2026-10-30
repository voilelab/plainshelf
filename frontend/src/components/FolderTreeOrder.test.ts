// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick, ref } from 'vue';
import { mount } from '../../testing/mount';
import FolderTree from './FolderTree.vue';

const metadataError = ref('');
const mocks = vi.hoisted(() => ({ save: vi.fn(), reload: vi.fn() }));
vi.mock('@/api/folders', () => ({ setFolderSortKey: mocks.save }));
vi.mock('@/composables/useBookStore', () => ({
  useBookStore: () => ({ books: ref([]) })
}));
vi.mock('@/composables/useFolderStore', () => ({
  useFolderStore: () => ({
    folderMeta: ref({}), folderMetaError: metadataError,
    error: ref(''),
    fetchFolders: mocks.reload
  })
}));
const nodes = [
  { path: 'a', name: 'a', children: [] },
  { path: 'b', name: 'b', children: [] }
];
const settle = async () => {
  for (let i = 0; i < 8; i++) await nextTick();
};
function drag(
  host: HTMLElement,
  source: string,
  target: string,
  sorting: boolean
) {
  const row =
    host.querySelectorAll('.folder-node')[
      nodes.findIndex((node) => node.path === target)
    ]!;
  const event = new Event('drop', { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'dataTransfer', {
    value: {
      getData: (mime: string) =>
        mime ===
        (sorting
          ? 'application/x-plainshelf-folder-sort'
          : 'application/x-plainshelf-folder-path')
          ? source
          : ''
    }
  });
  row.dispatchEvent(event);
}
describe('folder sorting gestures', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    metadataError.value = "";
    mocks.save.mockResolvedValue(undefined);
    mocks.reload.mockResolvedValue(undefined);
  });
  it('offers keyboard buttons, saves only metadata, and reloads after partial failure', async () => {
    const move = vi.fn();
    const { host } = mount(FolderTree, {
      props: { nodes, selected: undefined, onMoveFolder: move }
    });
    (host.querySelector('button[aria-pressed]') as HTMLButtonElement).click();
    await settle();
    mocks.save
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error('disk full'));
    (
      host.querySelector('button[aria-label="Move b up"]') as HTMLButtonElement
    ).click();
    await settle();
    expect(mocks.save.mock.calls).toEqual([
      ['b', 1024, '', ''],
      ['a', 2048, '', '']
    ]);
    expect(mocks.reload).toHaveBeenCalledOnce();
    expect(move).not.toHaveBeenCalled();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain(
      'Could not save'
    );
  });
  it('never turns sorting payloads or cross-parent sorting into directory moves', async () => {
    const move = vi.fn();
    const { host } = mount(FolderTree, {
      props: { nodes, selected: undefined, onMoveFolder: move }
    });
    drag(host, 'b', 'a', true);
    await settle();
    expect(mocks.save).not.toHaveBeenCalled();
    (host.querySelector('button[aria-pressed]') as HTMLButtonElement).click();
    await settle();
    drag(host, 'x/b', 'a', true);
    await settle();
    expect(mocks.save).not.toHaveBeenCalled();
    expect(move).not.toHaveBeenCalled();
    (host.querySelector('button[aria-pressed]') as HTMLButtonElement).click();
    await settle();
    drag(host, 'b', 'a', false);
    await settle();
    expect(move).toHaveBeenCalledWith({ folderPath: 'b', targetFolder: 'a' });
  });
  it('reports a metadata reload failure after writing instead of claiming success', async () => {
    const { host } = mount(FolderTree, {props: {nodes, selected: undefined}});
    (host.querySelector('button[aria-pressed]') as HTMLButtonElement).click();
    await settle();
    mocks.reload.mockImplementationOnce(async () => { metadataError.value = 'offline'; });
    (host.querySelector('button[aria-label="Move b up"]') as HTMLButtonElement).click();
    await settle();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Could not reload');
  });
  it('has no sorting controls or writes on read-only shelves', async () => {
    const { host } = mount(FolderTree, {
      props: { nodes, selected: undefined, readOnly: true }
    });
    expect(host.querySelector('button[aria-pressed]')).toBeNull();
    drag(host, 'b', 'a', true);
    await settle();
    expect(mocks.save).not.toHaveBeenCalled();
  });
});
