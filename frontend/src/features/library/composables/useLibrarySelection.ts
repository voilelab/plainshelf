import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue';
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';
import { useBookSelection } from '@/composables/useBookSelection';
import { useBookBatchOperations } from '@/composables/useBookBatchOperations';
import { handleLibraryMobileBack } from '@/features/library/utils/mobileBack';
import { retrySelection, runDownloadBatch } from '@/features/library/utils/downloadBatch';
import { useI18n } from '@/i18n';
import { getBookshelfProvider } from '@/providers';

interface LibrarySelectionOptions {
  books: Ref<Book[]>;
  visibleBookIds: Ref<string[]>;
  isMobileEnv: Ref<boolean>;
  readOnly: Ref<boolean>;
  openDetail: (id: string) => void;
  reloadBooks: () => Promise<void>;
}

/**
 * Multi-select on the library page and the batch actions it drives: move and
 * trash (desktop), download (mobile). Installs the keyboard shortcuts and the
 * Android back-button handler for the lifetime of the calling component.
 */
export function useLibrarySelection(options: LibrarySelectionOptions) {
  const { books, visibleBookIds, isMobileEnv, readOnly } = options;
  const { t } = useI18n();

  const selection = useBookSelection();
  const batchOperations = useBookBatchOperations();
  const moveBooksModalOpen = ref(false);
  const trashBooksModalOpen = ref(false);
  const downloadBatchOpen = ref(false);
  const downloadBatchRunning = ref(false);
  const downloadBatchPercentage = ref(0);
  const downloadBatchSucceeded = ref(0);
  const downloadBatchTotal = ref(0);
  const downloadBatchFailures = ref<Array<{ id: string; title: string; message: string }>>([]);
  const selectionEnabled = computed(() => isMobileEnv.value || !readOnly.value);
  const downloadBatchStatusText = computed(() => {
    if (downloadBatchRunning.value) return t('bookCollection.selection.processing');
    if (downloadBatchFailures.value.length === 0) {
      return t('bookCollection.selection.downloadComplete', { count: downloadBatchSucceeded.value });
    }
    return t('bookCollection.selection.downloadPartial', {
      succeeded: downloadBatchSucceeded.value,
      failed: downloadBatchFailures.value.length
    });
  });

  function selectedBooks(): Book[] {
    return books.value.filter((book) => selection.selectedIds.value.has(book.id));
  }

  function selectedTitles(): Record<string, string> {
    return Object.fromEntries(selectedBooks().map((book) => [book.id, book.title]));
  }

  function onBookActivate(payload: BookActivation): void {
    if (batchOperations.running.value || downloadBatchRunning.value) return;
    if (!selectionEnabled.value) {
      options.openDetail(payload.id);
      return;
    }
    if (!isMobileEnv.value && payload.shiftKey) {
      selection.selectRange(visibleBookIds.value, payload.id);
      return;
    }
    if (selection.active.value || payload.metaKey || payload.ctrlKey) {
      selection.toggle(payload.id);
      return;
    }
    options.openDetail(payload.id);
  }

  function onToggleSelection(id: string): void {
    if (!batchOperations.running.value && !downloadBatchRunning.value) selection.toggle(id);
  }

  function onLongPress(id: string): void {
    if (isMobileEnv.value && !downloadBatchRunning.value) selection.toggle(id);
  }

  function selectVisibleBooks(): void {
    if (!batchOperations.running.value && !downloadBatchRunning.value) selection.selectAll(visibleBookIds.value);
  }

  function openBatchMove(): void {
    if (!isMobileEnv.value && selection.active.value && !readOnly.value) moveBooksModalOpen.value = true;
  }

  function openBatchTrash(): void {
    if (!isMobileEnv.value && selection.active.value && !readOnly.value) trashBooksModalOpen.value = true;
  }

  function submitBatchMove(targetFolder: string): void {
    moveBooksModalOpen.value = false;
    const ids = [...selection.selectedIds.value];
    void batchOperations.startMove(ids, targetFolder.split('/').filter(Boolean), selectedTitles());
  }

  function submitBatchTrash(): void {
    trashBooksModalOpen.value = false;
    const ids = [...selection.selectedIds.value];
    void batchOperations.startTrash(ids, selectedTitles());
  }

  async function startBatchDownload(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.downloadBook || downloadBatchRunning.value) return;
    const targets = selectedBooks();
    if (targets.length === 0) return;

    downloadBatchOpen.value = true;
    downloadBatchRunning.value = true;
    downloadBatchPercentage.value = 0;
    downloadBatchSucceeded.value = 0;
    downloadBatchTotal.value = targets.length;
    downloadBatchFailures.value = [];

    // Called through the provider, not as a detached function: the mobile
    // provider's downloadBook is a method and needs its own `this`.
    const outcome = await runDownloadBatch(targets, (id) => provider.downloadBook!(id), {
      onProgress: (percentage) => {
        downloadBatchPercentage.value = percentage;
      },
      onFailure: (failure) => {
        downloadBatchFailures.value = [
          ...downloadBatchFailures.value,
          { ...failure, message: t('bookCollection.selection.failureCodes.download_failed') }
        ];
      }
    });
    downloadBatchSucceeded.value = outcome.succeeded;

    downloadBatchRunning.value = false;
    const failedVisible = retrySelection(outcome.failures, visibleBookIds.value);
    if (failedVisible.size > 0) selection.replace(failedVisible);
    else selection.clear();
    await options.reloadBooks();
  }

  function closeDownloadBatch(): void {
    if (!downloadBatchRunning.value) downloadBatchOpen.value = false;
  }

  function onSelectionKeydown(event: KeyboardEvent): void {
    if (!selection.active.value || batchOperations.running.value || downloadBatchRunning.value) return;
    const target = event.target;
    if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement || (target instanceof HTMLElement && target.isContentEditable)) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      selection.clear();
    } else if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'a') {
      event.preventDefault();
      selection.selectAll(visibleBookIds.value);
    }
  }

  let mobileBackHandle: { remove: () => Promise<void> } | null = null;

  async function installMobileBackHandler(): Promise<void> {
    if (!isMobileEnv.value) return;
    const { App } = await import('@capacitor/app');
    mobileBackHandle = await App.addListener('backButton', (event) => {
      handleLibraryMobileBack(event, {
        selectionActive: selection.active.value,
        downloadRunning: downloadBatchRunning.value,
        clearSelection: selection.clear,
        goBack: () => window.history.back(),
        exitApp: () => App.exitApp()
      });
    });
  }

  // A finished move/trash keeps only the visible books that failed selected, so
  // the user can retry them.
  watch(
    batchOperations.completionVersion,
    () => {
      const result = batchOperations.lastResult.value;
      if (!result) return;
      const failed = new Set(result.failures.map((failure) => failure.book_id).filter((id) => visibleBookIds.value.includes(id)));
      if (failed.size > 0) selection.replace(failed);
      else selection.clear();
    }
  );

  onMounted(() => {
    document.addEventListener('keydown', onSelectionKeydown);
    void installMobileBackHandler();
  });

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', onSelectionKeydown);
    void mobileBackHandle?.remove();
  });

  return {
    selection,
    batchOperations,
    selectionEnabled,
    moveBooksModalOpen,
    trashBooksModalOpen,
    downloadBatchOpen,
    downloadBatchRunning,
    downloadBatchPercentage,
    downloadBatchFailures,
    downloadBatchStatusText,
    onBookActivate,
    onToggleSelection,
    onLongPress,
    selectVisibleBooks,
    openBatchMove,
    openBatchTrash,
    submitBatchMove,
    submitBatchTrash,
    startBatchDownload,
    closeDownloadBatch
  };
}
