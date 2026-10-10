import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { hasFileTransfer, readDroppedFiles } from '@/utils/file';

interface LibraryImportOptions {
  readOnly: Ref<boolean>;
  // ?import=1 owns the import modal's open state (useBooksRouteQuery.ts).
  isImportModalOpen: Ref<boolean>;
  openImportModalQuery: () => Promise<unknown>;
  closeImportModalQuery: () => Promise<unknown>;
  reloadAfterImport: () => Promise<void>;
}

/**
 * The library page's ways into an import: the file picker, a document-wide
 * drop, and the new-empty-book modal. Installs the drag/drop listeners for the
 * lifetime of the calling component.
 */
export function useLibraryImport(options: LibraryImportOptions) {
  const { readOnly, isImportModalOpen, openImportModalQuery, closeImportModalQuery } = options;

  const isNewEmptyBookModalOpen = ref(false);
  const droppedFiles = ref<File[]>([]);
  // Host paths from the desktop native picker, handed to the import modal to
  // auto-start a per-file import. Empty on the web, where there is no picker.
  const desktopImportPaths = ref<string[]>([]);

  async function openImportFromFiles(): Promise<void> {
    if (readOnly.value) {
      return;
    }
    droppedFiles.value = [];
    desktopImportPaths.value = [];

    let desktopFiles: string[] | null = null;
    try {
      desktopFiles = await getBookshelfProvider().openLocalBookFiles?.() ?? null;
    } catch {
      desktopFiles = null;
    }

    // On the desktop the native picker returns the chosen host paths — an empty
    // array when the user cancelled. Hand them to the import modal, which
    // auto-starts a per-file import showing the same N/M progress and abort as the
    // browser upload. Off the desktop openLocalBookFiles is absent, so desktopFiles
    // is null and the ordinary browser file-input modal opens instead.
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

    // A drop is always a browser-File import, even on the desktop: clear any host
    // paths so the modal seeds from the dropped files rather than auto-starting a
    // stale picker selection.
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
