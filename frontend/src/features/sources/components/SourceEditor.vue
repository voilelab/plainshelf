<template>
  <section class="editor-panel">
    <div class="source-editor-status" role="status">
      <p v-if="loading" class="meta">Loading source...</p>
      <p v-else-if="!sourceId" class="meta">Select a source to start editing.</p>
      <p v-else-if="dirty" class="meta dirty">Unsaved changes</p>
      <p v-else class="meta">No pending changes</p>
    </div>

    <div class="editor-find-replace" role="group" aria-label="Find and replace">
      <label class="control-field">
        <span class="field-label">Find</span>
        <input
          v-model="findQuery"
          class="control-input"
          type="text"
          placeholder="Search text"
          :disabled="isEditorDisabled"
          @keydown.enter.prevent="findNext"
        />
      </label>

      <label class="control-field">
        <span class="field-label">Replace</span>
        <input
          v-model="replaceQuery"
          class="control-input"
          type="text"
          placeholder="Replace with"
          :disabled="isEditorDisabled"
          @keydown="onReplaceInputKeydown"
        />
      </label>

      <div class="find-actions">
        <button class="button" type="button" :disabled="disableFind" @click="findPrevious">Prev</button>
        <button class="button" type="button" :disabled="disableFind" @click="findNext">Next</button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceNext">
          Replace
        </button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceAll">
          Replace all
        </button>
      </div>
    </div>

    <div v-if="error" class="error editor-error" role="alert">{{ error }}</div>

    <textarea
      ref="textareaRef"
      class="source-content-textarea"
      :value="modelValue"
      :disabled="!sourceId || loading || saving"
      spellcheck="false"
      @input="onInput"
    ></textarea>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';

const props = defineProps<{
  modelValue: string;
  sourceId: string;
  loading?: boolean;
  saving?: boolean;
  dirty?: boolean;
  error?: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();

const textareaRef = ref<HTMLTextAreaElement | null>(null);
const findQuery = ref('');
const replaceQuery = ref('');

const isEditorDisabled = computed(() => !props.sourceId || props.loading || props.saving);
const disableFind = computed(() => isEditorDisabled.value || !findQuery.value);

watch(
  () => props.sourceId,
  () => {
    findQuery.value = '';
    replaceQuery.value = '';
  }
);

function onInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement;
  emit('update:modelValue', target.value);
}

function findNext(): void {
  findMatch(false);
}

function findPrevious(): void {
  findMatch(true);
}

function findMatch(backward: boolean): void {
  const textarea = textareaRef.value;
  const query = findQuery.value;
  if (!textarea || !query || isEditorDisabled.value) {
    return;
  }

  const text = textarea.value;
  let index = backward
    ? text.slice(0, textarea.selectionStart).lastIndexOf(query)
    : text.indexOf(query, textarea.selectionEnd);

  if (index === -1) {
    index = backward ? text.lastIndexOf(query) : text.indexOf(query);
  }

  if (index === -1) {
    return;
  }

  textarea.focus();
  textarea.setSelectionRange(index, index + query.length);
}

function replaceNext(): void {
  const textarea = textareaRef.value;
  const query = findQuery.value;
  if (!textarea || !query || isEditorDisabled.value) {
    return;
  }

  const selection = textarea.value.slice(textarea.selectionStart, textarea.selectionEnd);
  if (selection === query) {
    const before = textarea.value.slice(0, textarea.selectionStart);
    const after = textarea.value.slice(textarea.selectionEnd);
    const replaced = `${before}${replaceQuery.value}${after}`;
    const nextCursor = before.length + replaceQuery.value.length;
    emit('update:modelValue', replaced);
    void nextTick(() => {
      const current = textareaRef.value;
      if (!current) {
        return;
      }
      current.focus();
      current.setSelectionRange(nextCursor, nextCursor);
      findNext();
    });
    return;
  }

  findNext();
}

function onReplaceInputKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.isComposing || event.keyCode === 229) {
    return;
  }

  event.preventDefault();
  replaceNext();
}

function replaceAll(): void {
  const query = findQuery.value;
  if (!query || isEditorDisabled.value) {
    return;
  }

  const source = textareaRef.value?.value ?? props.modelValue;
  emit('update:modelValue', source.split(query).join(replaceQuery.value));
}
</script>

<style scoped>
.editor-panel {
  flex: 1;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.source-editor-status {
  flex-shrink: 0;
  min-height: 38px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  background: #f8fafc;
}

.source-editor-status p {
  margin: 0;
}

.source-editor-status .dirty {
  color: #9a3412;
}

.source-content-textarea {
  flex: 1;
  min-width: 0;
  min-height: 0;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  border: none;
  outline: none;
  background: #fff;
  color: var(--text);
  padding: 24px 32px;
  font-size: 16px;
  line-height: 1.7;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  resize: none;
  overflow: auto;
  white-space: pre;
  overflow-wrap: normal;
}

.source-content-textarea:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 32%, transparent);
  outline-offset: -2px;
}

.editor-find-replace {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 8px 10px;
  align-items: end;
  background: #f8fafc;
}

.control-field {
  display: grid;
  gap: 4px;
}

.field-label {
  font-size: 12px;
  color: var(--muted);
}

.control-input {
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0 10px;
  font: inherit;
}

.find-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.editor-error {
  margin: 10px 12px;
}

@media (max-width: 900px) {
  .editor-find-replace {
    grid-template-columns: 1fr;
  }

  .find-actions {
    flex-wrap: wrap;
  }
}
</style>
