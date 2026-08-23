<template>
  <section class="trash-page">
    <DeleteModal
      :open="pendingDeleteBook !== null"
      :title="t('trash.permanentDelete.title')"
      :item-name="pendingDeleteBook?.title ?? ''"
      :description="t('trash.permanentDelete.description')"
      :confirm-text="t('trash.permanentDelete.confirm')"
      :busy-text="t('trash.permanentDelete.busy')"
      :busy="Boolean(pendingDeleteBook && busyMap[pendingDeleteBook.id])"
      :error="actionError"
      @cancel="cancelPermanentDelete"
      @confirm="confirmPermanentDelete"
    />

    <ConfirmModal
      :open="emptyModalOpen"
      :title="t('trash.emptyAll.title')"
      :confirm-text="t('trash.emptyAll.confirm')"
      :cancel-text="emptyFinished ? t('trash.emptyAll.close') : t('common.cancel')"
      :busy-text="t('trash.emptyAll.busy')"
      :busy="emptying"
      :confirm-disabled="emptyFinished"
      :close-on-backdrop="!emptying"
      variant="danger"
      @cancel="closeEmptyModal"
      @confirm="confirmEmptyTrash"
    >
      <template v-if="!emptyStarted">
        <p>{{ emptyQuestionText }}</p>
        <p>{{ t('trash.emptyAll.description') }}</p>
      </template>
      <template v-else>
        <p>{{ emptyStatusText }}</p>
        <ProgressBar
          :value="emptyPercentage"
          :label="t('trash.emptyAll.progressLabel')"
        />
        <p class="progress-value">{{ Math.round(emptyPercentage) }}%</p>
      </template>
      <p v-if="emptyError" class="error" role="alert">{{ emptyError }}</p>
    </ConfirmModal>

    <header class="trash-header">
      <h2>{{ t('trash.title') }}</h2>
      <div class="header-actions">
        <button
          v-if="!readOnly"
          type="button"
          class="button danger"
          :disabled="loading || emptying"
          @click="requestEmptyTrash"
        >
          {{ t('trash.emptyAll.action') }}
        </button>
        <button type="button" class="button" :disabled="loading" @click="loadTrash">
          {{ t('common.retry') }}
        </button>
      </div>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-else-if="actionError && pendingDeleteBook === null" class="error" role="alert">{{ actionError }}</p>
    <p v-else-if="loading" class="loading">{{ t('trash.loading') }}</p>
    <p v-else-if="items.length === 0" class="loading">{{ t('trash.empty') }}</p>

    <template v-else>
      <table class="trash-table">
        <thead>
          <tr>
            <th>{{ t('trash.columns.title') }}</th>
            <th>{{ t('trash.columns.authors') }}</th>
            <th>{{ t('trash.columns.originalFolder') }}</th>
            <th>{{ t('trash.columns.originalPath') }}</th>
            <th>{{ t('trash.columns.deletedAt') }}</th>
            <th>{{ t('trash.columns.bookId') }}</th>
            <th>{{ t('trash.columns.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="book in visibleItems" :key="book.id">
            <td>{{ book.title }}</td>
            <td>{{ formatAuthors(book.authors) }}</td>
            <td>{{ formatFolder(book.original_folder) }}</td>
            <td>{{ book.original_path ?? '-' }}</td>
            <td>{{ formatDeletedAt(book.deleted_at) }}</td>
            <td class="book-id">{{ book.id }}</td>
            <td class="actions">
              <template v-if="!readOnly">
                <button
                  type="button"
                  class="button"
                  :disabled="Boolean(busyMap[book.id])"
                  @click="restore(book.id)"
                >
                  {{ t('trash.actions.restore') }}
                </button>
                <button
                  type="button"
                  class="button danger"
                  :disabled="Boolean(busyMap[book.id])"
                  @click="requestPermanentDelete(book)"
                >
                  {{ t('trash.actions.permanentDelete') }}
                </button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>

      <Pagination
        :page="page"
        :total="items.length"
        :page-size="pageSize"
        :page-size-options="PAGE_SIZE_OPTIONS"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import ConfirmModal from '@/components/ConfirmModal.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import Pagination from '@/components/Pagination.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { useBookCollectionRoute } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useI18n } from '@/i18n';
import type { TrashedBook } from '@/types/book';

const route = useRoute();
const { t } = useI18n();
const { writesEnabled } = useWriteAccess();
const readOnly = computed(() => !writesEnabled.value);
const { fetchBooks } = useBookStore();
const { fetchFolders } = useFolderStore();
const items = ref<TrashedBook[]>([]);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');
const actionError = ref('');
const pendingDeleteBook = ref<TrashedBook | null>(null);
const busyMap = ref<Record<string, boolean>>({});

function buildPageQuery(nextPage: number): Record<string, string> {
  return {
    ...route.query,
    page: String(nextPage)
  } as Record<string, string>;
}

// `items` stays the full trash listing: the empty state, the sweep confirmation
// count, and the total below all describe the whole trash, not the visible page.
const {
  page,
  pageSize,
  visibleBooks: visibleItems,
  onPageChange,
  onPageSizeChange,
  PAGE_SIZE_OPTIONS
} = useBookCollectionRoute<TrashedBook>({
  items,
  buildQuery: buildPageQuery,
  // The listing is fetched on mount, so clamping before it lands would drop a
  // deep-linked or reloaded `?page=N` back to page 1.
  clampEnabled: loaded
});

const emptyModalOpen = ref(false);

const {
  status: emptyStatus,
  percentage: emptyPercentage,
  error: emptyError,
  started: emptyStarted,
  running: emptying,
  finished: emptyFinished,
  start: startEmptyTrash,
  reset: resetEmptyProgress
} = useTaskChainProgress({
  onSettled: () => loadTrash(),
  startFailedMessage: () => t('trash.emptyAll.startFailed'),
  pollFailedMessage: () => t('trash.emptyAll.pollFailed')
});

// The listing hides books whose metadata cannot be read, so an empty table does
// not mean an empty trash — and those hidden directories are exactly what the
// sweep exists to remove. Only promise a count when one is actually known.
const emptyQuestionText = computed(() =>
  items.value.length > 0
    ? t('trash.emptyAll.question', { count: items.value.length })
    : t('trash.emptyAll.questionUnknownCount')
);

const emptyStatusText = computed(() => {
  switch (emptyStatus.value) {
    case 'completed':
      return t('trash.emptyAll.completed');
    case 'partially_completed':
      return t('trash.emptyAll.partiallyCompleted');
    case 'failed':
      return t('trash.emptyAll.failed');
    case 'running':
      return t('trash.emptyAll.running');
    default:
      return t('trash.emptyAll.pending');
  }
});

useDocumentTitle(() => [t('trash.title'), 'PlainShelf']);

function formatAuthors(authors: string[] | undefined): string {
  if (!authors || authors.length === 0) {
    return '-';
  }
  return authors.join(', ');
}

function formatFolder(folder: string[] | undefined): string {
  if (!folder || folder.length === 0) {
    return '/';
  }
  return folder.join('/');
}

function formatDeletedAt(value: string | undefined): string {
  if (!value) {
    return '-';
  }

  const time = new Date(value);
  if (Number.isNaN(time.getTime())) {
    return value;
  }
  return time.toLocaleString();
}

async function loadTrash(): Promise<void> {
  loading.value = true;
  error.value = '';
  actionError.value = '';
  try {
    items.value = await getBookshelfProvider().listTrashedBooks();
    loaded.value = true;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('trash.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function restore(id: string): Promise<void> {
  if (readOnly.value || busyMap.value[id]) {
    return;
  }

  actionError.value = '';
  busyMap.value = { ...busyMap.value, [id]: true };
  try {
    await bookshelfWriter().restoreTrashedBook(id);
    await Promise.all([loadTrash(), fetchBooks(), fetchFolders()]);
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('trash.restoreFailed');
  } finally {
    const { [id]: _ignored, ...rest } = busyMap.value;
    busyMap.value = rest;
  }
}

function requestPermanentDelete(book: TrashedBook): void {
  if (readOnly.value) {
    return;
  }

  actionError.value = '';
  pendingDeleteBook.value = book;
}

function cancelPermanentDelete(): void {
  if (pendingDeleteBook.value && busyMap.value[pendingDeleteBook.value.id]) {
    return;
  }
  pendingDeleteBook.value = null;
  actionError.value = '';
}

async function confirmPermanentDelete(): Promise<void> {
  const book = pendingDeleteBook.value;
  if (readOnly.value || !book || busyMap.value[book.id]) {
    return;
  }

  busyMap.value = { ...busyMap.value, [book.id]: true };
  actionError.value = '';
  try {
    await bookshelfWriter().deleteTrashedBook(book.id);
    pendingDeleteBook.value = null;
    await loadTrash();
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('trash.permanentDeleteFailed');
  } finally {
    const { [book.id]: _ignored, ...rest } = busyMap.value;
    busyMap.value = rest;
  }
}

function requestEmptyTrash(): void {
  if (readOnly.value) {
    return;
  }

  actionError.value = '';
  resetEmptyProgress();
  emptyModalOpen.value = true;
}

function closeEmptyModal(): void {
  // Closing mid-sweep would leave the user without any progress feedback, and
  // the task keeps running on the server regardless.
  if (emptying.value) {
    return;
  }

  resetEmptyProgress();
  emptyModalOpen.value = false;
}

async function confirmEmptyTrash(): Promise<void> {
  if (readOnly.value || emptyStarted.value) {
    return;
  }

  // A sweep already in flight returns its own ID, so this attaches to the
  // existing progress instead of scheduling a second one.
  await startEmptyTrash(() => bookshelfWriter().emptyTrash());
}

onMounted(() => {
  void loadTrash();
});
</script>

<style scoped>
.trash-page {
  padding: 24px 28px 32px;
}

.trash-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.trash-table {
  border-collapse: collapse;
  margin-top: 16px;
  width: 100%;
}

.trash-table th,
.trash-table td {
  border-bottom: 1px solid #e2e8f0;
  padding: 10px 8px;
  text-align: left;
  vertical-align: top;
}

.book-id {
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.actions {
  display: flex;
  gap: 8px;
}

.progress-value {
  color: #64748b;
  font-size: 13px;
  margin: 0;
  text-align: right;
}

.actions .button {
  font-size: 13px;
  padding: 6px 10px;
}

.button.danger {
  background: var(--danger, #dc2626);
  border-color: var(--danger, #dc2626);
  color: #fff;
}
</style>
