<template>
  <div v-if="open" class="modal-overlay" role="presentation" @click="onBackdropClick">
    <section
      class="panel import-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="import-modal-title"
      @click.stop
    >
      <header class="import-header">
        <h2 id="import-modal-title">Import Book</h2>
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

      <p class="meta">Upload a TXT or Markdown file to create a new book entry.</p>

      <div v-if="success" class="success">{{ success }}</div>
      <div v-if="error" class="error">{{ error }}</div>

      <form class="import-form" @submit.prevent="onSubmit">
        <label class="field">
          <span class="label">Book File (.txt / .md)</span>
          <input
            ref="bookInput"
            class="input file-input"
            type="file"
            accept=".txt,.md,text/plain,text/markdown"
            :disabled="submitting"
            multiple
            @change="onBookFileChange"
          />
        </label>

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
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useImportBook } from '../composables/useImportBook';
import { useBookStore } from '../composables/useBookStore';
import { useLayerStore } from '../composables/useLayerStore';
import { readSelectedFiles } from '../utils/file';

const props = defineProps<{
  open: boolean;
  currentLayerPath?: string;
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
  setBookFiles,
  submit,
  reset
} = useImportBook();
const { fetchBooks } = useBookStore();
const { fetchLayers } = useLayerStore();

const bookInput = ref<HTMLInputElement | null>(null);

function onBookFileChange(event: Event): void {
  setBookFiles(readSelectedFiles(event));
}

function clearFileInputs(): void {
  if (bookInput.value) {
    bookInput.value.value = '';
  }
}

function onClose(): void {
  if (submitting.value) {
    return;
  }
  emit('close');
}

function onBackdropClick(): void {
  onClose();
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (!props.open || submitting.value) {
    return;
  }
  if (event.key === 'Escape') {
    emit('close');
  }
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
    if (open) {
      return;
    }

    clearFileInputs();
    reset();
  }
);

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
});
</script>

<style scoped>
.modal-overlay {
  align-items: center;
  background: rgba(15, 23, 42, 0.38);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 16px;
  position: fixed;
  z-index: 50;
}

.import-modal {
  display: grid;
  gap: 10px;
  max-height: calc(100vh - 32px);
  overflow: auto;
  padding: 16px;
  width: min(100%, 620px);
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
    max-height: calc(100vh - 20px);
    padding: 14px;
  }
}
</style>
