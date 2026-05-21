<template>
  <div v-if="open" class="modal-overlay" role="presentation" @click="onBackdropClick">
    <section
      class="panel empty-book-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="empty-book-modal-title"
      @click.stop
    >
      <header class="modal-header">
        <h2 id="empty-book-modal-title">New empty book</h2>
        <button
          class="icon-close"
          type="button"
          aria-label="Close new empty book dialog"
          :disabled="submitting"
          @click="onClose"
        >
          ×
        </button>
      </header>

      <p class="meta">Create a new empty TXT book with title only.</p>

      <div v-if="error" class="error">{{ error }}</div>

      <form class="form" @submit.prevent="onSubmit">
        <label class="field" for="empty-book-title">
          <span class="label">Book Title</span>
          <input
            id="empty-book-title"
            ref="titleInput"
            v-model="title"
            class="input"
            type="text"
            :disabled="submitting"
            required
            maxlength="200"
            placeholder="Enter book title"
          />
        </label>

        <div class="actions">
          <button class="button" type="button" :disabled="submitting" @click="onClose">Cancel</button>
          <button class="button primary" type="submit" :disabled="submitting || !title.trim()">
            {{ submitting ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { importBook } from '../api/books';

const props = defineProps<{
  open: boolean;
  currentLayerPath?: string;
}>();

const emit = defineEmits<{
  close: [];
  imported: [{ successCount: number }];
}>();

const title = ref('');
const error = ref('');
const submitting = ref(false);
const titleInput = ref<HTMLInputElement | null>(null);

function reset(): void {
  title.value = '';
  error.value = '';
  submitting.value = false;
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
    onClose();
  }
}

async function onSubmit(): Promise<void> {
  const trimmedTitle = title.value.trim();
  if (!trimmedTitle || submitting.value) {
    return;
  }

  error.value = '';
  submitting.value = true;

  try {
    const emptyFile = new File([''], 'empty.txt', { type: 'text/plain' });
    await importBook({
      title: trimmedTitle,
      layer: props.currentLayerPath,
      file: emptyFile
    });
    emit('imported', { successCount: 1 });
    emit('close');
  } catch (err) {
    error.value = err instanceof Error && err.message ? err.message : 'Failed to create empty book.';
  } finally {
    submitting.value = false;
  }
}

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      reset();
      return;
    }

    await nextTick();
    titleInput.value?.focus();
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

.empty-book-modal {
  display: grid;
  gap: 10px;
  max-height: calc(100vh - 32px);
  overflow: auto;
  padding: 16px;
  width: min(100%, 460px);
}

.modal-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.modal-header h2 {
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

.field,
.form {
  display: grid;
  gap: 8px;
}

.label {
  color: var(--muted);
  font-size: 13px;
}

.actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 720px) {
  .empty-book-modal {
    width: 100%;
  }
}
</style>
