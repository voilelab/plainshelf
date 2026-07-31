<template>
  <section class="downloads-page">
    <DeleteModal
      :open="!!deleteTarget"
      :title="t('downloads.removeConfirm.title')"
      :item-name="deleteTarget?.title || ''"
      :description="t('downloads.removeConfirm.description')"
      :confirm-text="t('downloads.removeConfirm.confirm')"
      :busy-text="t('downloads.removeConfirm.busy')"
      :busy="removing"
      :error="actionError"
      @cancel="cancelRemove"
      @confirm="confirmRemove"
    />

    <header class="downloads-header">
      <div>
        <h2>{{ t('downloads.title') }}</h2>
        <p>{{ t('downloads.description') }}</p>
      </div>
      <button type="button" class="button" :disabled="loading" @click="loadEntries">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error && !loading" class="error" role="alert">{{ error }}</p>

    <section class="panel downloads-overview">
      <div class="downloads-overview-row">
        <span class="downloads-overview-label">{{ t('downloads.overview.countLabel') }}</span>
        <span class="downloads-overview-value">{{ t('downloads.overview.count', { count: entries.length }) }}</span>
      </div>
      <div class="downloads-overview-row">
        <span class="downloads-overview-label">{{ t('downloads.overview.totalSizeLabel') }}</span>
        <span class="downloads-overview-value">{{ formatBytes(totalSizeBytes) }}</span>
      </div>
      <div class="downloads-overview-row">
        <span class="downloads-overview-label">{{ t('downloads.overview.deviceStorageLabel') }}</span>
        <span v-if="storageEstimate && storageEstimate.supported" class="downloads-overview-value">
          {{ t('downloads.overview.storageUsed', { used: formatBytes(storageEstimate.usage ?? 0), quota: formatBytes(storageEstimate.quota ?? 0) }) }}
        </span>
        <span v-else class="downloads-overview-value downloads-overview-muted">
          {{ t('downloads.overview.storageUnsupported') }}
        </span>
      </div>
    </section>

    <p v-if="loading" class="loading">{{ t('downloads.list.loading') }}</p>
    <p v-else-if="entries.length === 0" class="loading">{{ t('downloads.list.empty') }}</p>

    <div v-else class="downloads-list">
      <article
        v-for="entry in entries"
        :key="entry.book.id"
        class="book-list-row panel downloads-row"
        @click="openBook(entry.book.id)"
      >
        <img
          :src="coverSrc(entry.book)"
          :alt="entry.book.title"
          class="book-list-cover"
          @error="onCoverError(entry.book.id)"
        />

        <div class="book-list-main">
          <div class="book-list-head">
            <h3 class="book-list-title">{{ entry.book.title }}</h3>
            <span class="downloads-size">{{ formatBytes(entry.sizeBytes) }}</span>
          </div>

          <p class="book-list-meta">
            <span v-if="entry.book.authors?.length">{{ entry.book.authors.join(', ') }}</span>
          </p>
        </div>

        <button
          type="button"
          class="downloads-remove-btn"
          :aria-label="t('downloads.item.remove')"
          @click.stop="requestRemove(entry.book)"
        >
          {{ t('downloads.item.remove') }}
        </button>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import DeleteModal from '@/components/DeleteModal.vue';
import bookcover from '@/assets/bookcover.svg';
import { getBookshelfProvider } from '@/providers';
import type { DownloadedBookEntry, StorageEstimateResult } from '@/providers/bookshelfProvider';
import type { Book } from '@/types/book';
import { formatBytes } from '@/utils/bytes';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';

const router = useRouter();
const { t } = useI18n();

const entries = ref<DownloadedBookEntry[]>([]);
const storageEstimate = ref<StorageEstimateResult | null>(null);
const loading = ref(false);
const error = ref('');
const deleteTarget = ref<Book | null>(null);
const removing = ref(false);
const actionError = ref('');
const brokenCoverIds = ref<Record<string, boolean>>({});

const totalSizeBytes = computed(() =>
  entries.value.reduce((total, entry) => total + entry.sizeBytes, 0)
);

useDocumentTitle(() => [t('downloads.title'), t('app.name')]);

function coverSrc(book: Book): string {
  if (brokenCoverIds.value[book.id]) {
    return bookcover;
  }
  return book.cover_url || bookcover;
}

function onCoverError(bookId: string): void {
  brokenCoverIds.value = {
    ...brokenCoverIds.value,
    [bookId]: true
  };
}

async function loadEntries(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const provider = getBookshelfProvider();
    const [nextEntries, nextEstimate] = await Promise.all([
      provider.listDownloadedBookEntries?.() ?? Promise.resolve([]),
      provider.getStorageEstimate?.() ?? Promise.resolve({ supported: false })
    ]);
    entries.value = nextEntries;
    storageEstimate.value = nextEstimate;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('downloads.list.loadFailed');
  } finally {
    loading.value = false;
  }
}

function openBook(id: string): void {
  void router.push(`/books/${id}`);
}

function requestRemove(book: Book): void {
  actionError.value = '';
  deleteTarget.value = book;
}

function cancelRemove(): void {
  if (removing.value) {
    return;
  }
  deleteTarget.value = null;
  actionError.value = '';
}

async function confirmRemove(): Promise<void> {
  const target = deleteTarget.value;
  const provider = getBookshelfProvider();
  if (!target || !provider.removeDownload || removing.value) {
    return;
  }

  removing.value = true;
  actionError.value = '';

  try {
    await provider.removeDownload(target.id);
    deleteTarget.value = null;
    await loadEntries();
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('downloads.removeConfirm.failed');
  } finally {
    removing.value = false;
  }
}

onMounted(() => {
  void loadEntries();
});
</script>

<style scoped>
.downloads-page {
  display: grid;
  gap: 16px;
}

.downloads-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.downloads-header h2 {
  margin: 0;
}

.downloads-header p {
  color: var(--muted);
  margin: 6px 0 0;
}

.downloads-overview {
  display: grid;
  gap: 8px;
}

.downloads-overview-row {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.downloads-overview-label {
  color: var(--muted);
  font-size: 13px;
}

.downloads-overview-value {
  font-weight: 600;
}

.downloads-overview-muted {
  color: var(--muted);
  font-weight: 400;
}

.downloads-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Row layout follows the BookListView.vue `.book-list-row` convention, with
   an extra column for the remove button (list rows are custom to this page
   because of the size + delete affordance, not a shared component). */
.book-list-row {
  align-items: center;
  cursor: pointer;
  display: grid;
  gap: 14px;
  padding: 10px 12px;
  transition: border-color 120ms ease, background 120ms ease, transform 120ms ease;
}

.book-list-row:hover {
  background: #fbfdff;
  border-color: #cdd8e6;
  transform: translateY(-1px);
}

.book-list-cover {
  width: 54px;
  height: 78px;
  border: 1px solid var(--border);
  border-radius: 10px;
  object-fit: cover;
  background: #edf2f7;
}

.book-list-main {
  min-width: 0;
}

.book-list-head {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.book-list-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
}

.book-list-meta {
  color: var(--muted);
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 6px 0 0;
  font-size: 12px;
}

.downloads-row {
  grid-template-columns: 54px minmax(0, 1fr) auto;
}

.downloads-size {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 12px;
  white-space: nowrap;
}

.downloads-remove-btn {
  background: transparent;
  border: 1px solid #fca5a5;
  border-radius: 6px;
  color: #b91c1c;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 6px 10px;
  white-space: nowrap;
}

.downloads-remove-btn:hover {
  background: #fef2f2;
}

@media (max-width: 760px) {
  .downloads-row {
    grid-template-columns: 44px minmax(0, 1fr) auto;
  }
}
</style>
