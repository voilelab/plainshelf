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

    <header class="trash-header">
      <h2>{{ t('trash.title') }}</h2>
      <button type="button" class="button" :disabled="loading" @click="loadTrash">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-else-if="actionError && pendingDeleteBook === null" class="error" role="alert">{{ actionError }}</p>
    <p v-else-if="loading" class="loading">{{ t('trash.loading') }}</p>
    <p v-else-if="items.length === 0" class="loading">{{ t('trash.empty') }}</p>

    <table v-else class="trash-table">
      <thead>
        <tr>
          <th>{{ t('trash.columns.title') }}</th>
          <th>{{ t('trash.columns.authors') }}</th>
          <th>{{ t('trash.columns.originalLayer') }}</th>
          <th>{{ t('trash.columns.deletedAt') }}</th>
          <th>{{ t('trash.columns.bookId') }}</th>
          <th>{{ t('trash.columns.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="book in items" :key="book.id">
          <td>{{ book.title }}</td>
          <td>{{ formatAuthors(book.authors) }}</td>
          <td>{{ formatLayer(book.original_layer) }}</td>
          <td>{{ formatDeletedAt(book.deleted_at) }}</td>
          <td class="book-id">{{ book.id }}</td>
          <td class="actions">
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
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import DeleteModal from '../components/DeleteModal.vue';
import { deleteTrashedBook, listTrashedBooks, restoreTrashedBook } from '../api/books';
import { useBookStore } from '../composables/useBookStore';
import { useLayerStore } from '../composables/useLayerStore';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useI18n } from '../i18n';
import type { TrashedBook } from '../types/book';

const { t } = useI18n();
const { fetchBooks } = useBookStore();
const { fetchLayers } = useLayerStore();
const items = ref<TrashedBook[]>([]);
const loading = ref(false);
const error = ref('');
const actionError = ref('');
const pendingDeleteBook = ref<TrashedBook | null>(null);
const busyMap = ref<Record<string, boolean>>({});

useDocumentTitle(() => [t('trash.title'), 'PlainShelf']);

function formatAuthors(authors: string[] | undefined): string {
  if (!authors || authors.length === 0) {
    return '-';
  }
  return authors.join(', ');
}

function formatLayer(layer: string[] | undefined): string {
  if (!layer || layer.length === 0) {
    return '/';
  }
  return layer.join('/');
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
    items.value = await listTrashedBooks();
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('trash.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function restore(id: string): Promise<void> {
  if (busyMap.value[id]) {
    return;
  }

  actionError.value = '';
  busyMap.value = { ...busyMap.value, [id]: true };
  try {
    await restoreTrashedBook(id);
    await Promise.all([loadTrash(), fetchBooks(), fetchLayers()]);
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('trash.restoreFailed');
  } finally {
    const { [id]: _ignored, ...rest } = busyMap.value;
    busyMap.value = rest;
  }
}

function requestPermanentDelete(book: TrashedBook): void {
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
  if (!book || busyMap.value[book.id]) {
    return;
  }

  busyMap.value = { ...busyMap.value, [book.id]: true };
  actionError.value = '';
  try {
    await deleteTrashedBook(book.id);
    pendingDeleteBook.value = null;
    await loadTrash();
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('trash.permanentDeleteFailed');
  } finally {
    const { [book.id]: _ignored, ...rest } = busyMap.value;
    busyMap.value = rest;
  }
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

.actions {
  display: flex;
  gap: 8px;
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
