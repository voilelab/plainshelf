// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  importBookFromLocalPath: vi.fn()
}));

vi.mock('@/providers/platformCapabilities', () => ({
  getPlatformCapabilities: () => ({
    importBookFromLocalPath: mocks.importBookFromLocalPath,
    onLocalFilesDropped: () => () => {}
  })
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => ({}),
  getBookshelfProvider: () => ({})
}));

vi.mock('@/composables/useBookStore', () => ({
  useBookStore: () => ({ fetchBooks: vi.fn(() => Promise.resolve()) })
}));

vi.mock('@/composables/useFolderStore', () => ({
  useFolderStore: () => ({ fetchFolders: vi.fn(() => Promise.resolve()) })
}));

vi.mock('@/api/settings', () => ({
  getEpubImportStrategySetting: () => new Promise(() => {})
}));

vi.mock('@/components/BaseDialog.vue', () => ({
  default: defineComponent({ name: 'BaseDialogStub', setup: (_, { slots }) => () => h('div', slots.default?.()) })
}));

import ImportBookModal from './ImportBookModal.vue';

const mounted: { app: App; host: HTMLElement }[] = [];

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
  mocks.importBookFromLocalPath.mockReset();
});

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function mountModal(initialPaths: string[]) {
  const localPaths = ref(initialPaths);
  const open = ref(false);
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({
    setup: () => () => h(ImportBookModal, { open: open.value, currentFolderPath: '/', localPaths: localPaths.value })
  });
  app.mount(host);
  mounted.push({ app, host });
  open.value = true;
  await nextTick();
  return { host, localPaths };
}

describe('ImportBookModal local-path drops', () => {
  it('imports a batch dropped while another is importing once that one finishes', async () => {
    const resolvers: Array<(value: { id: string }) => void> = [];
    mocks.importBookFromLocalPath.mockImplementation(
      () => new Promise((resolve) => { resolvers.push(resolve); })
    );
    const { host, localPaths } = await mountModal(['/books/a.txt']);
    await flush();
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(1);

    localPaths.value = ['/books/b.txt'];
    await nextTick();
    await flush();
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(1);

    resolvers[0]({ id: 'id-a' });
    await flush();
    await flush();
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(2);
    expect(mocks.importBookFromLocalPath).toHaveBeenLastCalledWith('/books/b.txt', '/');

    resolvers[1]({ id: 'id-b' });
    await flush();
    await flush();
    expect(mocks.importBookFromLocalPath).toHaveBeenCalledTimes(2);
    expect(host.querySelector('[data-file-drop-target]')).not.toBeNull();
  });
});
