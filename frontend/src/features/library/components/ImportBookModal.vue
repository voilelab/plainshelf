<template>
  <BaseDialog :open="open" :title="t('libraryForms.importBook.title')" :busy="submitting" @close="onClose">
    <section
      class="panel import-modal"
      :class="{ 'is-drop-target': isDropTarget }"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <header class="import-header">
        <h2>{{ t('libraryForms.importBook.title') }}</h2>
        <button
          class="icon-close"
          type="button"
          :aria-label="t('libraryForms.importBook.closeLabel')"
          :disabled="submitting"
          @click="onClose"
        >
          ×
        </button>
      </header>

      <p class="meta">{{ t('libraryForms.importBook.description') }}</p>

      <div v-if="success" class="success">{{ success }}</div>
      <div v-if="error" class="error">{{ error }}</div>

      <form class="import-form" @submit.prevent="onSubmit">
        <label class="field">
          <span class="label">{{ t('libraryForms.importBook.fileLabel') }}</span>
          <input
            ref="bookInput"
            class="input file-input"
            type="file"
            accept=".txt,.md,.epub,text/plain,text/markdown,application/epub+zip"
            :disabled="submitting"
            multiple
            @change="onBookFileChange"
          />
        </label>

        <section v-if="showEpubOptions" class="epub-options">
          <h3 class="epub-options-title">{{ t('libraryForms.importBook.epubTitle') }}</h3>
          <p class="meta">{{ t('libraryForms.importBook.epubDescription') }}</p>

          <label class="field">
            <span class="label">{{ t('libraryForms.importBook.convertTo') }}</span>
            <SelectRoot
              :model-value="epubStrategy.preset"
              :disabled="submitting"
              @update:model-value="onPresetSelect"
            >
              <SelectTrigger class="input select-trigger">
                <SelectValue />
              </SelectTrigger>
              <SelectPortal>
                <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                  <SelectViewport>
                    <SelectItem
                      v-for="opt in epubPresetOptions"
                      :key="opt.value"
                      class="reka-menu-item"
                      :value="opt.value"
                    >
                      <SelectItemText>{{ opt.label }}</SelectItemText>
                    </SelectItem>
                  </SelectViewport>
                </SelectContent>
              </SelectPortal>
            </SelectRoot>
          </label>

          <p v-if="epubStrategy.preset === 'plain'" class="meta epub-hint">
            {{ t('libraryForms.importBook.plainHint') }}
          </p>

          <label class="epub-checkbox">
            <input
              type="checkbox"
              :checked="epubStrategy.include_description"
              :disabled="submitting"
              @change="onIncludeDescriptionChange"
            />
            <span>{{ t('libraryForms.importBook.includeDescription') }}</span>
          </label>
        </section>

        <section v-if="showProgress" class="import-progress" aria-live="polite">
          <ProgressBar :value="progress.percentage" :label="progressText" />
          <p class="import-progress-text">{{ progressText }}</p>
        </section>

        <section v-if="files.length > 0" class="selected-files" aria-live="polite">
          <h3 class="selected-files-title">{{ t('libraryForms.importBook.selectedFiles') }}</h3>
          <ul class="file-list">
            <li v-for="(item, index) in files" :key="`${item.filename}-${index}`" class="file-item">
              <p class="file-name">{{ item.filename }}</p>
              <p class="file-meta">{{ t('libraryForms.importBook.fileTitle', { title: item.title }) }}</p>
              <p class="file-meta">
                {{ t('libraryForms.importBook.fileStatus') }}
                <span class="file-status" :class="`status-${item.status}`">{{ statusLabel(item.status) }}</span>
              </p>
              <p v-if="item.status === 'failed' && item.error" class="file-error">{{ item.error }}</p>
            </li>
          </ul>
        </section>

        <div class="actions">
          <button
            v-if="showProgress"
            class="button"
            type="button"
            :disabled="cancelRequested"
            @click="abort"
          >
            {{ cancelRequested ? t('libraryForms.importBook.aborting') : t('libraryForms.importBook.abort') }}
          </button>
          <button class="button" type="button" :disabled="submitting" @click="onClose">{{ t('common.cancel') }}</button>
          <button class="button primary" type="submit" :disabled="submitting || files.length === 0">
            {{ submitting ? t('libraryForms.importBook.submitting') : t('libraryForms.importBook.submit') }}
          </button>
        </div>
      </form>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import BaseDialog from '@/components/BaseDialog.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import { useImportBook, type ImportSubmitResult } from '@/features/library/composables/useImportBook';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { hasFileTransfer, readDroppedFiles, readSelectedFiles } from '@/utils/file';
import { getEpubImportStrategySetting } from '@/api/settings';
import type { EpubImportPreset } from '@/types/book';

import { useI18n } from '@/i18n';

const { t } = useI18n();

// A function, not a const array: the labels have to follow a locale change, and
// a module-level array resolves them once at import.
const epubPresetOptions = computed<{ value: EpubImportPreset; label: string }[]>(() => [
  { value: 'markdown', label: t('libraryForms.importBook.presetMarkdown') },
  { value: 'plain', label: t('libraryForms.importBook.presetPlain') }
]);

// The status is an enum token the UI used to render raw, so users saw
// "importing" rather than a sentence in their language.
function statusLabel(status: string): string {
  return t(`libraryForms.importBook.statuses.${status}`);
}

const props = defineProps<{
  open: boolean;
  currentFolderPath?: string;
  droppedFiles?: File[];
  // Desktop host paths from the native picker. When the modal opens with these
  // set, the import auto-starts through the shared executor — no file input and
  // no extra confirmation, matching the desktop's select-and-go flow.
  localPaths?: string[];
}>();

const emit = defineEmits<{
  close: [];
  imported: [{
    total: number;
    successCount: number;
    failedCount: number;
    cancelledCount: number;
    cancelled: boolean;
    firstImportedId?: string;
  }];
}>();

const {
  files,
  submitting,
  cancelRequested,
  success,
  error,
  epubStrategy,
  progress,
  setEpubStrategy,
  hasEpubFile,
  setBookFiles,
  submitFiles,
  submitLocalPaths,
  abort,
  reset
} = useImportBook();
const { fetchBooks } = useBookStore();
const { fetchFolders } = useFolderStore();

const bookInput = ref<HTMLInputElement | null>(null);
const isDropTarget = ref(false);
const selectedFiles = ref<File[]>([]);

// The conversion options only mean anything for EPUB, so they stay hidden until
// the selection actually contains one.
const showEpubOptions = computed(() => selectedFiles.value.length > 0 && hasEpubFile());

// A single-file import stays exactly as it was: no summary bar, no abort. The
// N / M progress only earns its space once a batch is being imported.
const showProgress = computed(() => submitting.value && progress.value.total > 1);

const progressText = computed(() =>
  t('libraryForms.importBook.progress', {
    current: progress.value.current,
    total: progress.value.total,
    filename: progress.value.filename
  })
);

function onBookFileChange(event: Event): void {
  const nextFiles = readSelectedFiles(event);
  selectedFiles.value = nextFiles;
  setBookFiles(nextFiles);
}

function onPresetSelect(value: AcceptableValue): void {
  setEpubStrategy({ ...epubStrategy.value, preset: value as EpubImportPreset });
}

function onIncludeDescriptionChange(event: Event): void {
  const target = event.target as HTMLInputElement;
  setEpubStrategy({ ...epubStrategy.value, include_description: target.checked });
}

// The configured default seeds the dialog; changing it here applies to this
// batch only and is not written back to the setting.
async function hydrateEpubStrategy(): Promise<void> {
  try {
    setEpubStrategy(await getEpubImportStrategySetting());
  } catch {
    // Keep the built-in default when the setting cannot be read.
  }
}

function clearFileInputs(): void {
  if (bookInput.value) {
    bookInput.value.value = '';
  }
}

function applyDroppedFiles(nextFiles: File[]): void {
  if (nextFiles.length === 0 || submitting.value) {
    return;
  }
  selectedFiles.value = nextFiles;
  setBookFiles(nextFiles);
  clearFileInputs();
}

function onClose(): void {
  if (submitting.value) {
    return;
  }
  emit('close');
}

function onDragOver(event: DragEvent): void {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
  if (!submitting.value) {
    isDropTarget.value = true;
  }
}

function onDragLeave(event: DragEvent): void {
  const relatedTarget = event.relatedTarget;
  const currentTarget = event.currentTarget;
  if (
    relatedTarget instanceof Node &&
    currentTarget instanceof Node &&
    currentTarget.contains(relatedTarget)
  ) {
    return;
  }

  isDropTarget.value = false;
}

function onDrop(event: DragEvent): void {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  isDropTarget.value = false;
  applyDroppedFiles(readDroppedFiles(event));
}

// Shared post-import handling for both the upload and desktop host-path flows:
// reload on any success, notify the page, then reset only on a fully clean run.
// A cancelled batch or one with any failure keeps its result message and
// per-file statuses on screen — the modal stays open (the page only reloads
// books), so resetting here would discard the very "N imported, rest cancelled"
// or "which files failed" summary the user needs. Only when every file imported
// is there nothing left to read, so the form clears for the next import.
async function finishImport(result: ImportSubmitResult | null): Promise<void> {
  if (!result) {
    return;
  }

  if (result.successCount > 0) {
    await Promise.all([fetchBooks(), fetchFolders()]);
  }

  emit('imported', result);

  if (result.cancelled || result.failedCount > 0) {
    return;
  }

  clearFileInputs();
  selectedFiles.value = [];
  reset();
}

async function onSubmit(): Promise<void> {
  await finishImport(await submitFiles(props.currentFolderPath));
}

// The desktop select-and-go path: the native picker already chose the files, so
// the modal opens straight into the shared progress/abort UI without a second
// confirmation. Guarded against re-entry so a re-render while importing cannot
// start a duplicate run.
async function runLocalPathsImport(): Promise<void> {
  const paths = props.localPaths ?? [];
  if (paths.length === 0 || submitting.value) {
    return;
  }
  await finishImport(await submitLocalPaths(paths, props.currentFolderPath));
}

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      return;
    }

    await nextTick();
    bookInput.value?.focus();
  }
);

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    void hydrateEpubStrategy();
  },
  { immediate: true }
);

watch(
  () => props.open,
  (open) => {
    if (open) {
      return;
    }

    isDropTarget.value = false;
    clearFileInputs();
    selectedFiles.value = [];
    reset();
  }
);

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    // A desktop host-path import takes over the modal on open; the browser
    // drag-drop seeding only applies when there are no host paths.
    if ((props.localPaths ?? []).length > 0) {
      return;
    }

    applyDroppedFiles(props.droppedFiles ?? []);
  }
);

watch(
  () => props.droppedFiles,
  (nextFiles) => {
    if (!props.open) {
      return;
    }

    applyDroppedFiles(nextFiles ?? []);
  }
);

// Desktop select-and-go: auto-start the import when the modal opens with host
// paths, and when a fresh selection arrives while it is already open.
watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    void runLocalPathsImport();
  }
);

watch(
  () => props.localPaths,
  () => {
    if (!props.open) {
      return;
    }

    void runLocalPathsImport();
  }
);
</script>

<style scoped>
.import-modal {
  display: grid;
  gap: 10px;
  max-height: calc(100vh / var(--app-zoom, 1) - 32px);
  overflow: auto;
  padding: 16px;
  width: min(100%, 620px);
}

.import-modal.is-drop-target {
  border: 2px dashed #1d4ed8;
  box-shadow: 0 0 0 3px rgba(29, 78, 216, 0.18);
}

.import-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.import-header h2 {
  margin: 0;
}

.icon-close {
  align-items: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  font-size: 20px;
  height: 32px;
  justify-content: center;
  line-height: 1;
  width: 32px;
}

.icon-close:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.field {
  display: grid;
  gap: 6px;
}

.label {
  color: var(--muted);
  font-size: 13px;
}

.import-form {
  display: grid;
  gap: 12px;
}

.file-input {
  padding-bottom: 7px;
  padding-top: 7px;
}

.actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 4px;
}

.import-progress {
  border: 1px solid var(--border);
  border-radius: 10px;
  display: grid;
  gap: 8px;
  padding: 10px;
}

.import-progress-text {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.selected-files {
  border: 1px solid var(--border);
  border-radius: 10px;
  display: grid;
  gap: 8px;
  padding: 10px;
}

.selected-files-title {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
}

.file-list {
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.file-item {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 8px 10px;
}

.file-name {
  font-weight: 600;
  margin: 0;
}

.file-meta {
  color: var(--muted);
  font-size: 12px;
  margin: 4px 0 0;
}

.file-status {
  font-weight: 600;
  text-transform: lowercase;
}

.status-pending {
  color: #475569;
}

.status-importing {
  color: #1d4ed8;
}

.status-success {
  color: #166534;
}

.status-failed {
  color: #b91c1c;
}

.status-cancelled {
  color: #92400e;
}

.file-error {
  color: #b91c1c;
  font-size: 12px;
  margin: 6px 0 0;
}

.success {
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  border-radius: 10px;
  color: #065f46;
  padding: 14px;
}

@media (max-width: 720px) {
  .import-modal {
    width: 100%;
    max-height: calc(100vh / var(--app-zoom, 1) - 20px);
    padding: 14px;
  }
}
</style>
