<template>
  <article class="duplicate-book-row">
    <DeleteModal
      :open="showDeleteModal"
      :item-name="book.title || book.id"
      :busy="deleting"
      :error="deleteError"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />

    <img :src="coverSrc" :alt="book.title" class="duplicate-cover" @error="onCoverError" />

    <div class="duplicate-meta">
      <h4 class="duplicate-title">{{ book.title }}</h4>
      <p class="duplicate-layer">{{ layerLabel }}</p>
    </div>

    <div class="duplicate-actions">
      <button type="button" class="button duplicate-open" :disabled="deleting" @click="openBook">{{ t('maintenance.duplicates.open') }}</button>
      <button
        type="button"
        class="button danger duplicate-delete"
        :disabled="deleting"
        :aria-label="t('maintenance.duplicates.deleteLabel', { title: book.title || t('maintenance.duplicates.untitledBook') })"
        @click="showDeleteModal = true"
      >
        {{ deleting ? t('maintenance.duplicates.deleting') : t('maintenance.duplicates.delete') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { bookshelfWriter } from '@/providers';
import type { Book } from '@/types/book';
import { getLayerPath, layerPathLabel } from '@/utils/layers';
import bookcover from '@/assets/bookcover.svg';
import DeleteModal from '@/components/DeleteModal.vue';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const props = defineProps<{
  book: Book;
}>();

const emit = defineEmits<{
  deleted: [bookId: string];
}>();

const router = useRouter();
const hasCoverLoadError = ref(false);
const showDeleteModal = ref(false);
const deleting = ref(false);
const deleteError = ref('');

const coverSrc = computed(() => {
  if (hasCoverLoadError.value) {
    return bookcover;
  }
  return props.book.cover_url || bookcover;
});

const layerLabel = computed(() => layerPathLabel(getLayerPath(props.book)));

watch(
  () => props.book.cover_url,
  () => {
    hasCoverLoadError.value = false;
  }
);

function onCoverError(): void {
  hasCoverLoadError.value = true;
}

function openBook(): void {
  void router.push(`/books/${props.book.id}`);
}

function cancelDelete(): void {
  if (deleting.value) {
    return;
  }
  showDeleteModal.value = false;
  deleteError.value = '';
}

async function confirmDelete(): Promise<void> {
  deleting.value = true;
  deleteError.value = '';

  try {
    await bookshelfWriter().deleteBook(props.book.id);
    showDeleteModal.value = false;
    emit('deleted', props.book.id);
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : t('maintenance.duplicates.deleteFailed');
  } finally {
    deleting.value = false;
  }
}
</script>

<style scoped>
.duplicate-book-row {
  align-items: center;
  border: 1px solid #e4ebf3;
  border-radius: 10px;
  display: grid;
  gap: 12px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  padding: 10px 12px;
}

.duplicate-cover {
  width: 42px;
  height: 62px;
  border-radius: 8px;
  border: 1px solid var(--border);
  object-fit: cover;
  background: #f4f7fb;
}

.duplicate-meta {
  min-width: 0;
}

.duplicate-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.3;
}

.duplicate-layer {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.3;
}

.duplicate-actions {
  display: flex;
  gap: 8px;
}

.duplicate-open {
  font-size: 13px;
  font-weight: 600;
  padding: 6px 12px;
}

@media (max-width: 700px) {
  .duplicate-book-row {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .duplicate-actions {
    grid-column: 1 / -1;
    justify-self: end;
  }

  .duplicate-open {
    justify-self: auto;
  }
}
</style>
