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
    <ConfirmModal
      :open="showDropConfirmModal"
      title="Update book cover?"
      confirm-text="Update cover"
      cancel-text="Cancel"
      :busy="coverBusy"
      busy-text="Uploading..."
      @cancel="cancelDroppedCover"
      @confirm="confirmDroppedCover"
    >
      <p>Do you want to update the book cover?</p>
      <p v-if="pendingCoverFile"><strong>{{ pendingCoverFile.name }}</strong></p>
      <p v-if="dropConfirmError" class="cover-modal-error" role="alert">{{ dropConfirmError }}</p>
    </ConfirmModal>
    <div
      class="cover-drop-target"
      :class="{ 'is-drag-over': isDragOver, 'is-busy': coverBusyAction === 'upload' }"
      :aria-busy="coverBusy ? 'true' : 'false'"
      @dragenter.prevent="onCoverDragEnter"
      @dragover.prevent="onCoverDragOver"
      @dragleave="onCoverDragLeave"
      @drop.prevent="onCoverDrop"
    >
      <img :src="resolvedCoverSrc" :alt="title" class="detail-cover" @error="onCoverError" />
      <div v-if="isDragOver || coverBusyAction === 'upload'" class="cover-drop-overlay">
        <div v-if="coverBusyAction === 'upload'" class="cover-spinner" aria-hidden="true"></div>
        <span>{{ coverBusyAction === 'upload' ? 'Uploading cover...' : 'Drop image to update cover' }}</span>
      </div>
    </div>
    <div class="cover-actions">
      <input
        ref="coverInputRef"
        class="cover-file-input"
        type="file"
        accept=".jpg,.jpeg,.png,.webp,.gif,image/jpeg,image/png,image/webp,image/gif"
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
import ConfirmModal from './ConfirmModal.vue';
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

const allowedCoverMimeTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
const coverExtPattern = /\.(jpg|jpeg|png|webp|gif)$/i;
const unsupportedCoverMessage = 'Only jpg, jpeg, png, webp, gif are supported.';

const coverInputRef = ref<HTMLInputElement | null>(null);
const coverBusy = ref(false);
const coverBusyAction = ref<'upload' | 'remove' | ''>('');
const coverStatus = ref('');
const coverError = ref(false);
const hasCoverLoadError = ref(false);
const coverCacheKey = ref<number | undefined>(undefined);
const showGenerateModal = ref(false);
const showDropConfirmModal = ref(false);
const isDragOver = ref(false);
const pendingCoverFile = ref<File | null>(null);
const dropConfirmError = ref('');

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
  if (source.includes('/api/shelves/') && source.includes('/books/') && source.includes('/cover')) {
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

function showUnsupportedCoverError(): void {
  coverStatus.value = unsupportedCoverMessage;
  coverError.value = true;
}

function hasDraggedFiles(event: DragEvent): boolean {
  const items = event.dataTransfer?.items;
  if (items && items.length > 0) {
    return Array.from(items).some((item) => item.kind === 'file');
  }
  return Boolean(event.dataTransfer?.files && event.dataTransfer.files.length > 0);
}

async function uploadCover(file: File): Promise<boolean> {
  if (!isSupportedCoverFile(file)) {
    showUnsupportedCoverError();
    clearCoverInput();
    return false;
  }

  coverBusy.value = true;
  coverBusyAction.value = 'upload';
  coverStatus.value = 'Uploading cover...';
  coverError.value = false;
  dropConfirmError.value = '';

  try {
    await uploadBookCover(props.bookId, file);
    hasCoverLoadError.value = false;
    coverCacheKey.value = Date.now();
    emit('cover-changed');
    coverStatus.value = 'Cover updated.';
    showDropConfirmModal.value = false;
    pendingCoverFile.value = null;
    return true;
  } catch (err) {
    const message = err instanceof Error ? `Upload failed: ${err.message}` : 'Upload failed';
    coverStatus.value = message;
    coverError.value = true;
    if (showDropConfirmModal.value) {
      dropConfirmError.value = message;
    }
    return false;
  } finally {
    coverBusy.value = false;
    coverBusyAction.value = '';
    clearCoverInput();
  }
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

  await uploadCover(file);
}

function onCoverDragEnter(event: DragEvent): void {
  if (coverBusy.value || !hasDraggedFiles(event)) {
    return;
  }
  isDragOver.value = true;
}

function onCoverDragOver(event: DragEvent): void {
  if (coverBusy.value || !hasDraggedFiles(event)) {
    return;
  }
  isDragOver.value = true;
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
}

function onCoverDragLeave(event: DragEvent): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (currentTarget instanceof HTMLElement && relatedTarget instanceof Node && currentTarget.contains(relatedTarget)) {
    return;
  }
  isDragOver.value = false;
}

function onCoverDrop(event: DragEvent): void {
  isDragOver.value = false;
  if (coverBusy.value) {
    return;
  }

  const file = event.dataTransfer?.files?.[0];
  if (!file) {
    return;
  }

  if (!isSupportedCoverFile(file)) {
    showUnsupportedCoverError();
    return;
  }

  pendingCoverFile.value = file;
  dropConfirmError.value = '';
  showDropConfirmModal.value = true;
}

function cancelDroppedCover(): void {
  if (coverBusy.value) {
    return;
  }
  showDropConfirmModal.value = false;
  pendingCoverFile.value = null;
  dropConfirmError.value = '';
}

async function confirmDroppedCover(): Promise<void> {
  const file = pendingCoverFile.value;
  if (!file || coverBusy.value) {
    return;
  }
  await uploadCover(file);
}

async function removeCover(): Promise<void> {
  if (!props.coverUrl || coverBusy.value) {
    return;
  }

  coverBusy.value = true;
  coverBusyAction.value = 'remove';
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
    coverBusyAction.value = '';
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

.cover-drop-target {
  position: relative;
}

.detail-cover {
  width: 100%;
  height: 260px;
  object-fit: cover;
  border-radius: 10px;
  border: 1px solid var(--border);
  display: block;
}

.cover-drop-target.is-drag-over .detail-cover {
  border-color: var(--primary, #2563eb);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.16);
}

.cover-drop-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 18px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.64);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  text-align: center;
  pointer-events: none;
}

.cover-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 999px;
  animation: cover-spin 0.8s linear infinite;
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

.cover-modal-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  padding: 10px;
}

@keyframes cover-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 780px) {
  .detail-cover {
    width: 110px;
    height: 160px;
    flex-shrink: 0;
  }
}
</style>