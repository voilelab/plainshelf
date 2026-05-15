<template>
  <section class="detail-shell">
    <DeleteModal
      :open="showDeleteModal"
      :item-name="book?.title || id"
      description="This cannot be undone."
      :busy="deleting"
      @cancel="showDeleteModal = false"
      @confirm="deleteBook"
    />
    <div v-if="showImportedMessage" class="loading">Book imported successfully.</div>
    <div v-if="showSavedMessage" class="loading">Metadata saved.</div>
    <div v-if="loading" class="loading">Loading book detail...</div>
    <div v-else-if="error" class="error detail-error" role="alert">
      <p>{{ error }}</p>
      <button class="button" type="button" @click="fetchDetail">Retry</button>
    </div>

    <article v-else-if="book" class="detail-panel">
      <div class="detail-cover-col">
        <BookCover
          :book-id="book.id"
          :title="book.title"
          :cover-url="book.cover_url"
          @cover-changed="onCoverChanged"
        />
      </div>

      <div>
        <BookDetail :book="book" :progress="progress" />
        <div class="actions">
          <button class="button primary" @click="goRead">Read</button>
          <button class="button" @click="goEditMetadata">Edit metadata</button>
          <button class="button" @click="goEditSnapshots">Edit Snapshots</button>
          <button class="button" @click="goLibrary">Back to books</button>
          <button class="button danger" :disabled="deleting" @click="confirmDelete">
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCover from '../components/BookCover.vue';
import BookDetail from '../components/BookDetail.vue';
import DeleteModal from '../components/DeleteModal.vue';
import { useBookDetail } from '../composables/useBookDetail';
import { useDocumentTitle } from '../composables/useDocumentTitle';

const route = useRoute();
const router = useRouter();
const id = computed(() => String(route.params.id));
const showImportedMessage = computed(() => route.query.imported === '1');
const showSavedMessage = computed(() => route.query.saved === '1');
const showDeleteModal = ref(false);

const {
  book,
  progress,
  loading,
  error,
  deleting,
  fetchDetail,
  removeBook
} = useBookDetail(() => id.value);

useDocumentTitle(() => ['Book', book.value?.title, 'PlainShelf']);

function goRead(): void {
  void router.push(`/reader/${id.value}`);
}

function goEditMetadata(): void {
  void router.push(`/books/${id.value}/edit`);
}

function goEditSnapshots(): void {
  void router.push(`/books/${id.value}/snapshots`);
}

function goLibrary(): void {
  void router.push('/books');
}

function onCoverChanged(): void {
  void fetchDetail();
}

function confirmDelete(): void {
  showDeleteModal.value = true;
}

async function deleteBook(): Promise<void> {
  const removed = await removeBook();
  if (removed) {
    showDeleteModal.value = false;
    await router.push('/books');
  }
}

watch(id, () => {
  void fetchDetail();
}, { immediate: true });
</script>

<style scoped>
.detail-shell {
  width: 100%;
  padding: 24px 28px 32px;
}

.detail-panel {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: 28px;
  align-items: start;
  width: 100%;
}

.detail-cover-col {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.detail-error {
  display: grid;
  gap: 10px;
}

.detail-error p {
  margin: 0;
}

.detail-error .button {
  justify-self: start;
}

.actions {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.actions .button {
  font-size: 18px;
  font-weight: 600;
  padding: 11px 18px;
  border-radius: 10px;
}

.actions .button.primary {
  padding-inline: 22px;
}

.button.danger {
  background: var(--danger, #dc2626);
  color: #fff;
  border-color: var(--danger, #dc2626);
}

.button.danger:hover:not(:disabled) {
  opacity: 0.85;
}

.detail-cover-col :deep(.cover-editor) {
  gap: 12px;
}

.detail-cover-col :deep(.detail-cover) {
  width: 100%;
  height: 420px;
  border-radius: 16px;
}

.detail-cover-col :deep(.cover-button-row) {
  gap: 10px;
}

.detail-cover-col :deep(.cover-btn) {
  font-size: 18px;
  font-weight: 600;
  padding: 10px 12px;
}

@media (max-width: 780px) {
  .detail-shell {
    padding: 18px 14px 24px;
  }

  .detail-panel {
    grid-template-columns: 1fr;
    gap: 18px;
  }

  .detail-cover-col {
    width: 100%;
    max-width: 340px;
  }

  .detail-cover-col :deep(.detail-cover) {
    height: 360px;
  }

  .meta-row {
    min-height: 56px;
    grid-template-columns: 1fr;
    gap: 6px;
    align-items: start;
  }
}
</style>
