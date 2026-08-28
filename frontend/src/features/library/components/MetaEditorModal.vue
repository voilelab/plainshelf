<template>
  <BaseDialog
    :open="open"
    :title="t('libraryForms.editBook.title')"
    :busy="saving"
    :described-by="descriptionId"
    @close="requestClose"
  >
    <section class="panel metadata-editor-modal" :aria-busy="loading || saving">
      <header class="metadata-editor-header">
        <div>
          <h2>{{ t('libraryForms.editBook.title') }}</h2>
          <p :id="descriptionId">{{ t('libraryForms.editBook.description') }}</p>
        </div>
        <button
          class="metadata-editor-close"
          type="button"
          :aria-label="t('libraryForms.editBook.closeLabel')"
          :disabled="saving"
          @click="requestClose"
        >
          ×
        </button>
      </header>

      <div class="metadata-editor-body">
        <div v-if="loading" class="metadata-editor-status" role="status">
          {{ t('libraryForms.editBook.loading') }}
        </div>
        <div v-else-if="loadError" class="metadata-editor-status metadata-editor-load-error" role="alert">
          <p>{{ loadError }}</p>
          <button class="button" type="button" @click="loadCurrentBook">
            {{ t('common.retry') }}
          </button>
        </div>
        <EditBook
          v-else-if="book"
          :book="book"
          :saving="saving"
          :error="saveError"
          embedded
          @submit="saveBook"
          @cancel="requestClose"
          @dirty-change="setDirty"
        />
      </div>
    </section>
  </BaseDialog>

  <ConfirmModal
    :open="showDiscardConfirmation"
    :title="t('libraryForms.editBook.discard.title')"
    :message="t('libraryForms.editBook.discard.message')"
    :confirm-text="t('libraryForms.editBook.discard.confirm')"
    :cancel-text="t('libraryForms.editBook.discard.cancel')"
    @cancel="showDiscardConfirmation = false"
    @confirm="discardAndClose"
  />
</template>

<script setup lang="ts">
import { ref, useId, watch } from 'vue';
import BaseDialog from '@/components/BaseDialog.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import EditBook from '@/features/library/components/EditBook.vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book, BookUpdateRequest } from '@/types/book';
import { useI18n } from '@/i18n';

const props = defineProps<{
  open: boolean;
  bookId: string | null;
}>();

const emit = defineEmits<{
  close: [];
  submit: [payload: BookUpdateRequest];
  saved: [book: Book];
  'dirty-change': [dirty: boolean];
}>();

const { t } = useI18n();
const descriptionId = `metadata-editor-description-${useId()}`;
const book = ref<Book | null>(null);
const loading = ref(false);
const saving = ref(false);
const loadError = ref('');
const saveError = ref('');
const dirty = ref(false);
const showDiscardConfirmation = ref(false);
let session = 0;

// The modal owns close-button/backdrop protection, while its host page owns
// route, unload, and native-back protection. Relay changes synchronously so a
// browser/native back action cannot unmount the draft between watcher flushes.
function setDirty(value: boolean): void {
  dirty.value = value;
  emit('dirty-change', value);
}

watch(
  () => [props.open, props.bookId] as const,
  ([open]) => {
    session += 1;
    showDiscardConfirmation.value = false;
    setDirty(false);
    saving.value = false;
    saveError.value = '';

    if (!open) {
      loading.value = false;
      loadError.value = '';
      book.value = null;
      return;
    }

    void loadCurrentBook();
  },
  { immediate: true }
);

/**
 * The modal owns this read deliberately. List and detail pages may be showing a
 * cached Book, while metadata can also change on disk outside PlainShelf. Every
 * open (and every retry) therefore asks the active provider for the latest copy.
 */
async function loadCurrentBook(): Promise<void> {
  const bookId = props.bookId;
  const request = (session += 1);
  book.value = null;
  setDirty(false);
  loading.value = true;
  loadError.value = '';
  saveError.value = '';

  if (!bookId) {
    loading.value = false;
    loadError.value = t('libraryForms.editBook.loadFailed');
    return;
  }

  try {
    const latestBook = await getBookshelfProvider().getBook(bookId);
    if (request !== session || !props.open || props.bookId !== bookId) {
      return;
    }
    book.value = latestBook;
  } catch (error) {
    if (request !== session || !props.open || props.bookId !== bookId) {
      return;
    }
    loadError.value = error instanceof Error
      ? error.message
      : t('libraryForms.editBook.loadFailed');
  } finally {
    if (request === session) {
      loading.value = false;
    }
  }
}

function requestClose(): void {
  if (saving.value) {
    return;
  }
  if (dirty.value) {
    showDiscardConfirmation.value = true;
    return;
  }
  emit('close');
}

function discardAndClose(): void {
  showDiscardConfirmation.value = false;
  setDirty(false);
  emit('close');
}

async function saveBook(payload: BookUpdateRequest): Promise<void> {
  const currentBook = book.value;
  if (!currentBook || saving.value) {
    return;
  }

  const request = session;
  saving.value = true;
  saveError.value = '';
  emit('submit', payload);

  try {
    const updatedBook = await bookshelfWriter().updateBook(currentBook.id, payload);
    if (request !== session || !props.open || props.bookId !== currentBook.id) {
      return;
    }
    book.value = updatedBook;
    setDirty(false);
    emit('saved', updatedBook);
    emit('close');
  } catch (error) {
    if (request !== session || !props.open || props.bookId !== currentBook.id) {
      return;
    }
    saveError.value = error instanceof Error
      ? error.message
      : t('libraryForms.editBook.saveFailed');
  } finally {
    if (request === session) {
      saving.value = false;
    }
  }
}
</script>

<style scoped>
.metadata-editor-modal {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  width: min(860px, calc(100vw / var(--app-zoom, 1) - 32px));
  max-height: calc(100vh / var(--app-zoom, 1) - 32px);
  overflow: hidden;
}

.metadata-editor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px 14px;
  border-bottom: 1px solid var(--border);
}

.metadata-editor-header h2,
.metadata-editor-header p,
.metadata-editor-load-error p {
  margin: 0;
}

.metadata-editor-header h2 {
  font-size: 20px;
  line-height: 1.25;
}

.metadata-editor-header p {
  margin-top: 4px;
  color: var(--muted);
  font-size: 13px;
}

.metadata-editor-close {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font-size: 21px;
  line-height: 1;
}

.metadata-editor-close:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.metadata-editor-body {
  min-height: 0;
  padding: 16px 20px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.metadata-editor-status {
  display: grid;
  min-height: 160px;
  place-content: center;
  color: var(--muted);
  text-align: center;
}

.metadata-editor-load-error {
  gap: 12px;
}

.metadata-editor-load-error .button {
  justify-self: center;
}

@media (max-width: 520px) {
  .metadata-editor-modal {
    width: calc(100vw / var(--app-zoom, 1) - 16px);
    max-height: calc(100vh / var(--app-zoom, 1) - 16px);
  }

  .metadata-editor-header {
    padding: 14px;
  }

  .metadata-editor-body {
    padding: 12px 14px;
  }
}
</style>
