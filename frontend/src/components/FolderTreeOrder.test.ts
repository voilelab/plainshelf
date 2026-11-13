// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick, ref } from 'vue';
import { mount } from '../../testing/mount';
import { setLocale } from '@/i18n';
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
  { path: 'b', name: 'b', children: [] },
  { path: 'c', name: 'c', children: [] }
];
const ROW_HEIGHT = 40;
const settle = async () => {
  for (let i = 0; i < 8; i++) await nextTick();
};
function row(host: HTMLElement, path: string): HTMLElement {
  const element = host.querySelectorAll<HTMLElement>('.folder-node')[
    nodes.findIndex((node) => node.path === path)
  ]!;
  element.getBoundingClientRect = () => ({ top: 0, height: ROW_HEIGHT }) as DOMRect;
  return element;
}
function dragEvent(type: string, source: string, offsetY: number): Event {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'clientY', { value: offsetY });
  Object.defineProperty(event, 'dataTransfer', {
    value: {
      getData: (mime: string) =>
        mime === 'application/x-plainshelf-folder-path' ? source : '',
      setData: () => undefined
    }
  });
  return event;
}
function drop(host: HTMLElement, source: string, target: string, offsetY: number) {
  row(host, target).dispatchEvent(dragEvent('drop', source, offsetY));
}
async function chooseMenuItem(host: HTMLElement, path: string, label: string) {
  row(host, path).dispatchEvent(
    new MouseEvent('contextmenu', { bubbles: true, clientX: 10, clientY: 10 })
  );
  await settle();
  const item = [...document.body.querySelectorAll<HTMLElement>('[role="menuitem"]')].find(
    (element) => element.textContent?.trim() === label
  );
  if (!item) throw new Error(`no menu item ${label}`);
  item.click();
  await settle();
}
describe('folder drop zones', () => {
  beforeEach(() => {
    setLocale('en');
    vi.clearAllMocks();
    metadataError.value = '';
    mocks.save.mockResolvedValue(undefined);
    mocks.reload.mockResolvedValue(undefined);
  });
  it('sorts before a sibling dropped on its top edge', async () => {
    const move = vi.fn();
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined, onMoveFolder: move } });
    drop(host, 'c', 'a', 2);
    await settle();
    expect(mocks.save.mock.calls).toEqual([
      ['c', 1024, '', ''],
      ['a', 2048, '', ''],
      ['b', 3072, '', '']
    ]);
    expect(move).not.toHaveBeenCalled();
  });
  it('sorts after a sibling dropped on its bottom edge', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined } });
    drop(host, 'a', 'c', ROW_HEIGHT - 2);
    await settle();
    expect(mocks.save.mock.calls.map((call) => call[0])).toEqual(['b', 'c', 'a']);
  });
  it('moves into a folder dropped on its middle', async () => {
    const move = vi.fn();
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined, onMoveFolder: move } });
    drop(host, 'c', 'a', ROW_HEIGHT / 2);
    await settle();
    expect(move).toHaveBeenCalledWith({ folderPath: 'c', targetFolder: 'a' });
    expect(mocks.save).not.toHaveBeenCalled();
  });
  it('moves a non-sibling into the folder even on an edge', async () => {
    const move = vi.fn();
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined, onMoveFolder: move } });
    drop(host, 'x/b', 'a', 2);
    await settle();
    expect(move).toHaveBeenCalledWith({ folderPath: 'x/b', targetFolder: 'a' });
    expect(mocks.save).not.toHaveBeenCalled();
  });
  it('draws an insertion line only for a sibling drag over an edge', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined } });
    row(host, 'c').dispatchEvent(dragEvent('dragstart', 'c', 0));
    row(host, 'a').dispatchEvent(dragEvent('dragover', 'c', 2));
    await settle();
    expect(row(host, 'a').classList).toContain('drop-before');
    row(host, 'a').dispatchEvent(dragEvent('dragover', 'c', ROW_HEIGHT / 2));
    await settle();
    expect(row(host, 'a').classList).toContain('drop-target');
    expect(row(host, 'a').classList).not.toContain('drop-before');
  });
  it('writes nothing for a drop that keeps the order', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined } });
    drop(host, 'a', 'b', 2);
    await settle();
    expect(mocks.save).not.toHaveBeenCalled();
  });
  it('moves up from the menu and reloads after a partial failure', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined } });
    mocks.save
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error('disk full'));
    await chooseMenuItem(host, 'b', 'Move up');
    expect(mocks.save.mock.calls.slice(0, 2)).toEqual([
      ['b', 1024, '', ''],
      ['a', 2048, '', '']
    ]);
    expect(mocks.reload).toHaveBeenCalledOnce();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Could not save');
  });
  it('reports a metadata reload failure after writing', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined } });
    mocks.reload.mockImplementationOnce(async () => { metadataError.value = 'offline'; });
    await chooseMenuItem(host, 'c', 'Move up');
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Could not reload');
  });
  it('has no sorting menu or writes on read-only shelves', async () => {
    const { host } = mount(FolderTree, { props: { nodes, selected: undefined, readOnly: true } });
    drop(host, 'c', 'a', 2);
    await settle();
    expect(mocks.save).not.toHaveBeenCalled();
    row(host, 'b').dispatchEvent(
      new MouseEvent('contextmenu', { bubbles: true, clientX: 10, clientY: 10 })
    );
    await settle();
    const labels = [...document.body.querySelectorAll('[role="menuitem"]')].map((item) => item.textContent?.trim());
    expect(labels).not.toContain('Move up');
  });
});
