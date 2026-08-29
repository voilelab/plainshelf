<template>
  <div class="cover-editor">
    <GenerateCoverModal
      :open="showGenerateModal && !readOnly"
      :book-id="bookId"
      :initial-title="title"
      :initial-author="authorText"
      @close="showGenerateModal = false"
      @saved="onGeneratedCoverSaved"
    />
    <ConfirmModal
      :open="showDropConfirmModal && !readOnly"
      :title="t('bookDetail.cover.confirmTitle')"
      :confirm-text="t('bookDetail.cover.confirm')"
      :cancel-text="t('common.cancel')"
      :busy="coverBusy"
      :busy-text="t('bookDetail.cover.uploading')"
      @cancel="cancelDroppedCover"
      @confirm="confirmDroppedCover"
    >
      <p>{{ t('bookDetail.cover.confirmQuestion') }}</p>
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
      <BookCoverImg
        :book-id="bookId"
        :cover-url="coverUrl"
        :alt="title"
        :cache-key="coverCacheKey"
        class="detail-cover"
      />
      <div v-if="isDragOver || coverBusyAction === 'upload'" class="cover-drop-overlay">
        <div v-if="coverBusyAction === 'upload'" class="cover-spinner" aria-hidden="true"></div>
        <span>{{ coverBusyAction === 'upload' ? t('bookDetail.cover.uploading') : t('bookDetail.cover.dropHint') }}</span>
      </div>
    </div>
    <div v-if="!readOnly" class="cover-actions">
      <input
        ref="coverInputRef"
        class="cover-file-input"
        type="file"
        accept=".jpg,.jpeg,.png,.webp,.gif,image/jpeg,image/png,image/webp,image/gif"
        @change="onCoverFileChange"
      />
      <CollapsibleRoot v-model:open="showCoverOptions" class="cover-options-collapsible">
        <CollapsibleTrigger class="button cover-options-toggle" :disabled="coverBusy">
          {{ t('bookDetail.cover.options') }}
          <span aria-hidden="true">{{ showCoverOptions ? '−' : '+' }}</span>
        </CollapsibleTrigger>
        <CollapsibleContent class="cover-action-tray" :unmount-on-hide="false">
          <div class="cover-button-row">
            <button class="button cover-btn" type="button" :disabled="coverBusy" @click="openPicker">
              {{ coverBusy ? '…' : t('bookDetail.cover.upload') }}
            </button>
            <button class="button cover-btn" type="button" :disabled="coverBusy || !coverUrl" @click="removeCover">
              {{ t('bookDetail.cover.remove') }}
            </button>
          </div>
          <div class="cover-button-row">
            <button class="button cover-btn" type="button" :disabled="coverBusy" @click="showGenerateModal = true">
              {{ t('bookDetail.cover.generate') }}
            </button>
          </div>
        </CollapsibleContent>
      </CollapsibleRoot>
      <p v-if="coverStatus" class="cover-status" :class="{ error: coverError }">{{ coverStatus }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui';
import { computed, ref } from 'vue';
import { bookshelfWriter } from '@/providers';
import BookCoverImg from '@/components/BookCoverImg.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import GenerateCoverModal from './GenerateCoverModal.vue';
import { useI18n } from '@/i18n';

const props = defineProps<{
  bookId: string;
  title: string;
  authors?: string[];
  coverUrl?: string;
  readOnly?: boolean;
}>();

const emit = defineEmits<{
  (event: 'cover-changed'): void;
}>();

const allowedCoverMimeTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
const coverExtPattern = /\.(jpg|jpeg|png|webp|gif)$/i;
const { t } = useI18n();

const coverInputRef = ref<HTMLInputElement | null>(null);
const coverBusy = ref(false);
const coverBusyAction = ref<'upload' | 'remove' | ''>('');
const coverStatus = ref('');
const coverError = ref(false);
const coverCacheKey = ref<number | undefined>(undefined);
const showGenerateModal = ref(false);
const showCoverOptions = ref(false);
const showDropConfirmModal = ref(false);
const isDragOver = ref(false);
const pendingCoverFile = ref<File | null>(null);
const dropConfirmError = ref('');

const authorText = computed(() => {
  if (!props.authors || props.authors.length === 0) return '';
  return props.authors.join(', ');
});

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
  coverStatus.value = t('bookDetail.cover.unsupported');
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
  if (props.readOnly) {
    return false;
  }
  if (!isSupportedCoverFile(file)) {
    showUnsupportedCoverError();
    clearCoverInput();
    return false;
  }

  coverBusy.value = true;
  coverBusyAction.value = 'upload';
  coverStatus.value = t('bookDetail.cover.uploading');
  coverError.value = false;
  dropConfirmError.value = '';

  try {
    await bookshelfWriter().uploadBookCover(props.bookId, file);
    coverCacheKey.value = Date.now();
    emit('cover-changed');
    coverStatus.value = t('bookDetail.cover.updated');
    showDropConfirmModal.value = false;
    pendingCoverFile.value = null;
    return true;
  } catch (err) {
    const message = err instanceof Error
      ? t('bookDetail.cover.uploadFailedWithReason', { reason: err.message })
      : t('bookDetail.cover.uploadFailed');
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
  if (props.readOnly) {
    return;
  }
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
  if (props.readOnly) {
    return;
  }
  if (coverBusy.value || !hasDraggedFiles(event)) {
    return;
  }
  isDragOver.value = true;
}

function onCoverDragOver(event: DragEvent): void {
  if (props.readOnly) {
    return;
  }
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
  if (props.readOnly) {
    return;
  }
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
  if (props.readOnly) {
    return;
  }
  if (!props.coverUrl || coverBusy.value) {
    return;
  }

  coverBusy.value = true;
  coverBusyAction.value = 'remove';
  coverStatus.value = t('bookDetail.cover.removing');
  coverError.value = false;

  try {
    await bookshelfWriter().deleteBookCover(props.bookId);
    coverCacheKey.value = undefined;
    emit('cover-changed');
    coverStatus.value = t('bookDetail.cover.removed');
  } catch (err) {
    coverStatus.value = err instanceof Error
      ? t('bookDetail.cover.removeFailedWithReason', { reason: err.message })
      : t('bookDetail.cover.removeFailed');
    coverError.value = true;
  } finally {
    coverBusy.value = false;
    coverBusyAction.value = '';
  }
}

function onGeneratedCoverSaved(): void {
  coverCacheKey.value = Date.now();
  emit('cover-changed');
  coverStatus.value = t('bookDetail.cover.updated');
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
  aspect-ratio: 2 / 3;
  height: auto;
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
  gap: 8px;
}

.cover-options-toggle {
  align-items: center;
  background: rgba(255, 255, 255, 0.66);
  color: #596676;
  display: flex;
  font-size: 13px;
  font-weight: 650;
  justify-content: space-between;
  min-height: 44px;
  width: 100%;
}

/* Groups the toggle and its tray for reka without becoming a grid item of the
   cover-actions grid; the trigger and tray stay direct rows as before. */
.cover-options-collapsible {
  display: contents;
}

.cover-action-tray {
  display: grid;
  gap: 6px;
}

/* reka keeps the tray mounted (:unmount-on-hide="false") and marks it hidden
   when closed; the author display:grid above would otherwise beat the
   user-agent [hidden] rule and leave the tray visible. */
.cover-action-tray[hidden] {
  display: none;
}

.cover-button-row {
  display: flex;
  gap: 6px;
}

.cover-btn {
  flex: 1;
  font-size: 12px;
  min-height: 44px;
  padding: 7px 8px;
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
</style>
