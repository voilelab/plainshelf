import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue';
import { hasFileTransfer, readDroppedFiles } from '@/utils/file';

interface LibraryImportOptions {
  readOnly: Ref<boolean>;
  isImportModalOpen: Ref<boolean>;
  openImportModalQuery: () => Promise<unknown>;
  closeImportModalQuery: () => Promise<unknown>;
  reloadAfterImport: () => Promise<void>;
}

export function useLibraryImport(options: LibraryImportOptions) {
  const { readOnly, isImportModalOpen, openImportModalQuery, closeImportModalQuery } = options;

  const isNewEmptyBookModalOpen = ref(false);
  const droppedFiles = ref<File[]>([]);
  const desktopImportPaths = ref<string[]>([]);

  async function openImportFromFiles(): Promise<void> {
    if (readOnly.value) {
      return;
    }
    droppedFiles.value = [];
    desktopImportPaths.value = [];

    let desktopFiles: string[] | null = null;
    try {
      desktopFiles = await getPlatformCapabilities().openLocalBookFiles?.() ?? null;
    } catch {
      desktopFiles = null;
    }

    if (desktopFiles) {
      if (desktopFiles.length === 0) {
        return;
      }
      desktopImportPaths.value = desktopFiles;
    }

    if (isImportModalOpen.value) {
      return;
    }

    void openImportModalQuery();
  }

  function openNewEmptyBookModal(): void {
    if (readOnly.value) {
      return;
    }
    isNewEmptyBookModalOpen.value = true;
  }

  function closeNewEmptyBookModal(): void {
    isNewEmptyBookModalOpen.value = false;
  }

  function onDocumentDragOver(event: DragEvent): void {
    if (readOnly.value) {
      return;
    }
    if (!hasFileTransfer(event.dataTransfer)) {
      return;
    }

    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'copy';
    }
  }

  function onDocumentDrop(event: DragEvent): void {
    if (readOnly.value) {
      return;
    }
    if (!hasFileTransfer(event.dataTransfer)) {
      return;
    }

    event.preventDefault();
    const nextDroppedFiles = readDroppedFiles(event);
    if (nextDroppedFiles.length === 0) {
      return;
    }

    desktopImportPaths.value = [];
    droppedFiles.value = nextDroppedFiles;
    if (!isImportModalOpen.value) {
      void openImportModalQuery();
    }
  }

  function closeImportModal(): void {
    if (!isImportModalOpen.value) {
      return;
    }

    droppedFiles.value = [];
    desktopImportPaths.value = [];
    void closeImportModalQuery();
  }

  async function onImported(result: { successCount: number }): Promise<void> {
    if (result.successCount > 0) {
      await options.reloadAfterImport();
    }
  }

  onMounted(() => {
    document.addEventListener('dragover', onDocumentDragOver);
    document.addEventListener('drop', onDocumentDrop);
  });

  onBeforeUnmount(() => {
    document.removeEventListener('dragover', onDocumentDragOver);
    document.removeEventListener('drop', onDocumentDrop);
  });

  return {
    isNewEmptyBookModalOpen,
    droppedFiles,
    desktopImportPaths,
    openImportFromFiles,
    openNewEmptyBookModal,
    closeNewEmptyBookModal,
    closeImportModal,
    onImported
  };
}
