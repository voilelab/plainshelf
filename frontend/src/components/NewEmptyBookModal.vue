<template>
  <BaseDialog :open="open" title="New empty book" :busy="submitting" @close="onClose">
    <section class="panel empty-book-modal">
      <header class="modal-header">
        <h2>New empty book</h2>
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
  </BaseDialog>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import BaseDialog from './BaseDialog.vue';
import { getBookshelfProvider } from '@/providers';

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

async function onSubmit(): Promise<void> {
  const trimmedTitle = title.value.trim();
  if (!trimmedTitle || submitting.value) {
    return;
  }

  error.value = '';
  submitting.value = true;

  try {
    const emptyFile = new File([''], 'empty.txt', { type: 'text/plain' });
    await getBookshelfProvider().importBook({
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
</script>

<style scoped>
.empty-book-modal {
  display: grid;
  gap: 10px;
  max-height: calc(100vh / var(--app-zoom, 1) - 32px);
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
