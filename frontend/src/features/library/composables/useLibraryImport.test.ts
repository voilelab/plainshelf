// @vitest-environment jsdom
import { createApp, defineComponent, h, ref } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

type DroppedFiles = { paths: string[]; unsupported: string[] };

const mocks = vi.hoisted(() => ({
  openLocalBookFiles: undefined as undefined | (() => Promise<string[]>),
  onLocalFilesDropped: undefined as undefined | ((handler: (drop: DroppedFiles) => void) => () => void)
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ openLocalBookFiles: mocks.openLocalBookFiles })
}));

import { useLibraryImport } from './useLibraryImport';

type Api = ReturnType<typeof useLibraryImport>;

let host: HTMLElement | null = null;
let app: ReturnType<typeof createApp> | null = null;

function mountImport(readOnly = false) {
  const isImportModalOpen = ref(false);
  const openImportModalQuery = vi.fn(() => {
    isImportModalOpen.value = true;
    return Promise.resolve();
  });
  const reloadAfterImport = vi.fn(() => Promise.resolve());
  let api!: Api;
  app = createApp(defineComponent({
    setup() {
      api = useLibraryImport({
        readOnly: ref(readOnly),
        isImportModalOpen,
        openImportModalQuery,
        closeImportModalQuery: vi.fn(() => Promise.resolve()),
        reloadAfterImport
      });
      return () => h('div');
    }
  }));
  host = document.createElement('div');
  app.mount(host);
  return { api, openImportModalQuery, reloadAfterImport };
}

// jsdom 30.1.1 does not implement DataTransfer in its living interface registry.
function dropEvent(type: 'dragover' | 'drop', files: File[]): DragEvent {
  const event = new Event(type, { cancelable: true }) as DragEvent;
  Object.defineProperty(event, 'dataTransfer', {
    value: { types: ['Files'], files, items: [], dropEffect: 'none' }
  });
  return event;
}

function unmount(): void {
  app?.unmount();
  app = null;
  host = null;
}

afterEach(() => {
  unmount();
  mocks.openLocalBookFiles = undefined;
  mocks.onLocalFilesDropped = undefined;
});

function stubNativeFileDrop() {
  const native = { handler: null as null | ((drop: DroppedFiles) => void), stop: vi.fn() };
  mocks.onLocalFilesDropped = (handler) => {
    native.handler = handler;
    return native.stop;
  };
  return native;
}

describe('useLibraryImport', () => {
  it('opens the import modal seeded with files dropped anywhere on the document', () => {
    const { api, openImportModalQuery } = mountImport();
    const file = new File(['x'], 'a.txt');

    const event = dropEvent('drop', [file]);
    document.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(api.droppedFiles.value).toEqual([file]);
    expect(openImportModalQuery).toHaveBeenCalledTimes(1);
  });

  it('ignores drops while read-only and after unmount', () => {
    const { openImportModalQuery } = mountImport(true);
    document.dispatchEvent(dropEvent('drop', [new File(['x'], 'a.txt')]));
    expect(openImportModalQuery).not.toHaveBeenCalled();

    unmount();
    const live = mountImport();
    unmount();
    document.dispatchEvent(dropEvent('drop', [new File(['x'], 'b.txt')]));
    expect(live.openImportModalQuery).not.toHaveBeenCalled();
  });

  it('hands desktop picker paths to the modal and stays closed when the picker is cancelled', async () => {
    mocks.openLocalBookFiles = vi.fn().mockResolvedValueOnce([]).mockResolvedValueOnce(['/tmp/a.txt']);
    const { api, openImportModalQuery } = mountImport();

    await api.openImportFromFiles();
    expect(openImportModalQuery).not.toHaveBeenCalled();

    await api.openImportFromFiles();
    expect(api.desktopImportPaths.value).toEqual(['/tmp/a.txt']);
    expect(openImportModalQuery).toHaveBeenCalledTimes(1);
  });

  it('imports natively dropped book paths through the modal and leaves the document drop alone', () => {
    const native = stubNativeFileDrop();
    const { api, openImportModalQuery } = mountImport();

    expect(api.nativeFileDrop).toBe(true);
    const htmlDrop = dropEvent('drop', [new File(['x'], 'a.txt')]);
    document.dispatchEvent(htmlDrop);
    expect(htmlDrop.defaultPrevented).toBe(false);
    expect(openImportModalQuery).not.toHaveBeenCalled();

    native.handler!({ paths: ['/books/a.txt', '/books/b.epub'], unsupported: [] });
    expect(api.desktopImportPaths.value).toEqual(['/books/a.txt', '/books/b.epub']);
    expect(api.dropError.value).toBe('');
    expect(openImportModalQuery).toHaveBeenCalledTimes(1);

    unmount();
    expect(native.stop).toHaveBeenCalledTimes(1);
  });

  it('reports unsupported dropped files by name and imports nothing when none are books', () => {
    const native = stubNativeFileDrop();
    const { api, openImportModalQuery } = mountImport();

    native.handler!({ paths: [], unsupported: ['/books/scan.pdf', '/books/Series'] });

    expect(openImportModalQuery).not.toHaveBeenCalled();
    expect(api.desktopImportPaths.value).toEqual([]);
    expect(api.dropError.value).toContain('scan.pdf, Series');
  });

  it('ignores native drops on a read-only shelf', () => {
    const native = stubNativeFileDrop();
    const { api, openImportModalQuery } = mountImport(true);

    native.handler!({ paths: ['/books/a.txt'], unsupported: ['/books/b.pdf'] });

    expect(openImportModalQuery).not.toHaveBeenCalled();
    expect(api.desktopImportPaths.value).toEqual([]);
    expect(api.dropError.value).toBe('');
  });

  it('reloads only when an import added at least one book', async () => {
    const { api, reloadAfterImport } = mountImport();

    await api.onImported({ successCount: 0 });
    expect(reloadAfterImport).not.toHaveBeenCalled();
    await api.onImported({ successCount: 2 });
    expect(reloadAfterImport).toHaveBeenCalledTimes(1);
  });
});

vi.mock('@/providers/platformCapabilities', () => ({
  getPlatformCapabilities: () => ({
    openLocalBookFiles: mocks.openLocalBookFiles,
    onLocalFilesDropped: mocks.onLocalFilesDropped
  })
}));
