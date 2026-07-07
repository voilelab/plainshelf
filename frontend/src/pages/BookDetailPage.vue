<template>
  <section class="detail-shell">
    <DeleteModal
      :open="!!deleteTarget"
      :item-name="deleteTarget?.title || id"
      description="The book will be moved to Trash. You can restore it later."
      :busy="deleting"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />
    <div v-if="showImportedMessage" class="loading">Book imported successfully.</div>
    <div v-if="showSavedMessage" class="loading">Metadata saved.</div>
    <div v-if="actionError" class="error detail-error" role="alert">
      <p>{{ actionError }}</p>
      <button class="button" type="button" @click="dismissActionError">Dismiss</button>
    </div>
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
          :authors="book.authors"
          :cover-url="book.cover_url"
          :read-only="readOnly"
          @cover-changed="onCoverChanged"
        />
      </div>

      <div>
        <BookDetail :book="book" :progress="progress" :current-source="currentSource" />
        <div class="actions">
          <button class="button primary" @click="goRead(id)">Read</button>
          <button class="button" :disabled="downloading" @click="downloadBook(book)">
            {{ downloading ? 'Downloading...' : 'Download' }}
          </button>
          <button v-if="canOpenBookFolder" class="button" @click="openBookFolder(id)">Open Folder</button>
          <button v-if="!readOnly" class="button" @click="goEdit(id)">Edit metadata</button>
          <button v-if="!readOnly" class="button" @click="goEditSources">Edit Sources</button>
          <button v-if="!readOnly" class="button danger" :disabled="deleting" @click="onRequestDelete">
            {{ deleting ? 'Moving...' : 'Move to Trash' }}
          </button>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCover from '../components/BookCover.vue';
import BookDetail from '../components/BookDetail.vue';
import DeleteModal from '../components/DeleteModal.vue';
import { useBookActions } from '../composables/useBookActions';
import { useBookDetail } from '../composables/useBookDetail';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useServerMode } from '../composables/useServerMode';

const route = useRoute();
const router = useRouter();
const id = computed(() => String(route.params.id));
const showImportedMessage = computed(() => route.query.imported === '1');
const showSavedMessage = computed(() => route.query.saved === '1');
const { readOnly } = useServerMode();

const {
  book,
  progress,
  currentSource: currentSource,
  loading,
  error,
  fetchDetail
} = useBookDetail(() => id.value);

const {
  downloading,
  actionError,
  deleteTarget,
  deleting,
  canOpenBookFolder,
  goRead,
  goEdit,
  openBookFolder,
  downloadBook,
  requestDelete,
  cancelDelete,
  confirmDelete,
  dismissActionError
} = useBookActions({
  onDeleted: () => {
    void router.push('/books');
  }
});

useDocumentTitle(() => ['Book', book.value?.title, 'PlainShelf']);

function goEditSources(): void {
  if (readOnly.value) {
    return;
  }
  void router.push(`/books/${id.value}/sources`);
}

function onCoverChanged(): void {
  void fetchDetail();
}

function onRequestDelete(): void {
  if (readOnly.value || !book.value) {
    return;
  }
  requestDelete(book.value);
}

watch(id, () => {
  dismissActionError();
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
