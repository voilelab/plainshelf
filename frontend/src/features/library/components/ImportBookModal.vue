<template>
  <BaseDialog :open="open" title="Import Book" :busy="submitting" @close="onClose">
    <section
      class="panel import-modal"
      :class="{ 'is-drop-target': isDropTarget }"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <header class="import-header">
        <h2>Import Book</h2>
        <button
          class="icon-close"
          type="button"
          aria-label="Close import dialog"
          :disabled="submitting"
          @click="onClose"
        >
          ×
        </button>
      </header>

      <p class="meta">Upload a TXT, Markdown or EPUB file to create a new book entry, or drag-and-drop files here.</p>

      <div v-if="success" class="success">{{ success }}</div>
      <div v-if="error" class="error">{{ error }}</div>

      <form class="import-form" @submit.prevent="onSubmit">
        <label class="field">
          <span class="label">Book File (.txt, .md, .epub)</span>
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
          <h3 class="epub-options-title">EPUB Conversion</h3>
          <p class="meta">
            EPUB files are converted to text. The original file is not kept and embedded
            illustrations are dropped.
          </p>

          <label class="field">
            <span class="label">Convert to</span>
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
                      v-for="opt in EPUB_PRESET_OPTIONS"
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
            Plain text has no heading marker to match on, so the reader lists chapters as
            "Part 1", "Part 2" and so on. Markdown keeps the chapter names.
          </p>

          <label class="epub-checkbox">
            <input
              type="checkbox"
              :checked="epubStrategy.include_description"
              :disabled="submitting"
              @change="onIncludeDescriptionChange"
            />
            <span>Put the book description at the start of the text</span>
          </label>
        </section>

        <section v-if="files.length > 0" class="selected-files" aria-live="polite">
          <h3 class="selected-files-title">Selected Files</h3>
          <ul class="file-list">
            <li v-for="(item, index) in files" :key="`${item.filename}-${index}`" class="file-item">
              <p class="file-name">{{ item.filename }}</p>
              <p class="file-meta">Title: {{ item.title }}</p>
              <p class="file-meta">
                Status:
                <span class="file-status" :class="`status-${item.status}`">{{ item.status }}</span>
              </p>
              <p v-if="item.status === 'failed' && item.error" class="file-error">{{ item.error }}</p>
            </li>
          </ul>
        </section>

        <div class="actions">
          <button class="button" type="button" :disabled="submitting" @click="onClose">Cancel</button>
          <button class="button primary" type="submit" :disabled="submitting || files.length === 0">
            {{ submitting ? 'Importing...' : 'Import' }}
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
import { useImportBook } from '@/features/library/composables/useImportBook';
import { useBookStore } from '@/composables/useBookStore';
import { useLayerStore } from '@/composables/useLayerStore';
import { hasFileTransfer, readDroppedFiles, readSelectedFiles } from '@/utils/file';
import { getEpubImportStrategySetting } from '@/api/settings';
import type { EpubImportPreset } from '@/types/book';

const EPUB_PRESET_OPTIONS: { value: EpubImportPreset; label: string }[] = [
  { value: 'markdown', label: 'Markdown (chapter headings)' },
  { value: 'plain', label: 'Plain text' }
];

const props = defineProps<{
  open: boolean;
  currentLayerPath?: string;
  droppedFiles?: File[];
}>();

const emit = defineEmits<{
  close: [];
  imported: [{
    total: number;
    successCount: number;
    failedCount: number;
    firstImportedId?: string;
  }];
}>();

const {
  files,
  submitting,
  success,
  error,
  epubStrategy,
  setEpubStrategy,
  hasEpubFile,
  setBookFiles,
  submit,
  reset
} = useImportBook();
const { fetchBooks } = useBookStore();
const { fetchLayers } = useLayerStore();

const bookInput = ref<HTMLInputElement | null>(null);
const isDropTarget = ref(false);
const selectedFiles = ref<File[]>([]);

// The conversion options only mean anything for EPUB, so they stay hidden until
// the selection actually contains one.
const showEpubOptions = computed(() => selectedFiles.value.length > 0 && hasEpubFile());

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

async function onSubmit(): Promise<void> {
  const result = await submit(props.currentLayerPath);
  if (!result) {
    return;
  }

  if (result.successCount > 0) {
    await Promise.all([fetchBooks(), fetchLayers()]);
  }

  emit('imported', result);

  clearFileInputs();
  selectedFiles.value = [];
  reset();
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
