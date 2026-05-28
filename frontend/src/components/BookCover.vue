<template>
  <div class="cover-editor">
    <GenerateCoverModal
      :open="showGenerateModal"
      :book-id="bookId"
      :initial-title="title"
      :initial-author="authorText"
      @close="showGenerateModal = false"
      @saved="onGeneratedCoverSaved"
    />
    <img :src="resolvedCoverSrc" :alt="title" class="detail-cover" @error="onCoverError" />
    <div class="cover-actions">
      <input
        ref="coverInputRef"
        class="cover-file-input"
        type="file"
        accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"
        @change="onCoverFileChange"
      />
      <div class="cover-button-row">
        <button class="button cover-btn" :disabled="coverBusy" @click="openPicker">
          {{ coverBusy ? '...' : 'Upload' }}
        </button>
        <button class="button cover-btn" :disabled="coverBusy || !coverUrl" @click="removeCover">
          Remove
        </button>
      </div>
      <div class="cover-button-row">
        <button class="button cover-btn" :disabled="coverBusy" @click="showGenerateModal = true">
          Generate cover
        </button>
      </div>
      <p v-if="coverStatus" class="cover-status" :class="{ error: coverError }">{{ coverStatus }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { deleteBookCover, getBookCoverUrl, uploadBookCover } from '../api/books';
import bookcover from '../assets/bookcover.svg';
import GenerateCoverModal from './GenerateCoverModal.vue';

const props = defineProps<{
  bookId: string;
  title: string;
  authors?: string[];
  coverUrl?: string;
}>();

const emit = defineEmits<{
  (event: 'cover-changed'): void;
}>();

const allowedCoverMimeTypes = ['image/jpeg', 'image/png', 'image/webp'];
const coverExtPattern = /\.(jpg|jpeg|png|webp)$/i;

const coverInputRef = ref<HTMLInputElement | null>(null);
const coverBusy = ref(false);
const coverStatus = ref('');
const coverError = ref(false);
const hasCoverLoadError = ref(false);
const coverCacheKey = ref<number | undefined>(undefined);
const showGenerateModal = ref(false);

const authorText = computed(() => {
  if (!props.authors || props.authors.length === 0) return '';
  return props.authors.join(', ');
});

const resolvedCoverSrc = computed(() => {
  if (hasCoverLoadError.value) {
    return bookcover;
  }
  const source = props.coverUrl;
  if (!source) {
    return bookcover;
  }
  if (source.includes('/api/books/') && source.includes('/cover')) {
    return getBookCoverUrl(props.bookId, coverCacheKey.value);
  }
  return source;
});

watch(
  () => props.coverUrl,
  () => {
    hasCoverLoadError.value = false;
  }
);

function onCoverError(): void {
  hasCoverLoadError.value = true;
}

function clearCoverInput(): void {
  if (coverInputRef.value) {
    coverInputRef.value.value = '';
  }
}

function isSupportedCoverFile(file: File): boolean {
  if (allowedCoverMimeTypes.includes(file.type)) {
    return true;
  }
  return coverExtPattern.test(file.name);
}

function openPicker(): void {
  if (coverBusy.value) {
    return;
  }
  coverInputRef.value?.click();
}

async function onCoverFileChange(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) {
    return;
  }

  if (!isSupportedCoverFile(file)) {
    coverStatus.value = 'Only jpg, jpeg, png, webp are supported.';
    coverError.value = true;
    clearCoverInput();
    return;
  }

  coverBusy.value = true;
  coverStatus.value = 'Uploading cover...';
  coverError.value = false;

  try {
    await uploadBookCover(props.bookId, file);
    hasCoverLoadError.value = false;
    coverCacheKey.value = Date.now();
    emit('cover-changed');
    coverStatus.value = 'Cover updated.';
  } catch (err) {
    coverStatus.value = err instanceof Error ? `Upload failed: ${err.message}` : 'Upload failed';
    coverError.value = true;
  } finally {
    coverBusy.value = false;
    clearCoverInput();
  }
}

async function removeCover(): Promise<void> {
  if (!props.coverUrl || coverBusy.value) {
    return;
  }

  coverBusy.value = true;
  coverStatus.value = 'Removing cover...';
  coverError.value = false;

  try {
    await deleteBookCover(props.bookId);
    hasCoverLoadError.value = false;
    coverCacheKey.value = undefined;
    emit('cover-changed');
    coverStatus.value = 'Cover removed.';
  } catch (err) {
    coverStatus.value = err instanceof Error ? `Remove failed: ${err.message}` : 'Remove failed';
    coverError.value = true;
  } finally {
    coverBusy.value = false;
  }
}

function onGeneratedCoverSaved(): void {
  hasCoverLoadError.value = false;
  coverCacheKey.value = Date.now();
  emit('cover-changed');
  coverStatus.value = 'Cover updated.';
  coverError.value = false;
}
</script>

<style scoped>
.cover-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-cover {
  width: 100%;
  height: 260px;
  object-fit: cover;
  border-radius: 10px;
  border: 1px solid var(--border);
  display: block;
}

.cover-actions {
  display: grid;
  gap: 6px;
}

.cover-button-row {
  display: flex;
  gap: 6px;
}

.cover-btn {
  flex: 1;
  font-size: 12px;
  padding: 4px 8px;
}

.cover-file-input {
  display: none;
}

.cover-status {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}

.cover-status.error {
  color: var(--danger, #dc2626);
}

@media (max-width: 780px) {
  .detail-cover {
    width: 110px;
    height: 160px;
    flex-shrink: 0;
  }
}
</style>