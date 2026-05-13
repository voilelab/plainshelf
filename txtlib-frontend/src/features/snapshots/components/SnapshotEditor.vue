<template>
  <section class="editor-panel">
    <div class="snapshot-editor-status" role="status">
      <p v-if="loading" class="meta">Loading snapshot...</p>
      <p v-else-if="!snapshotId" class="meta">Select a snapshot to start editing.</p>
      <p v-else-if="dirty" class="meta dirty">Unsaved changes</p>
      <p v-else class="meta">No pending changes</p>
    </div>

    <div v-if="error" class="error editor-error" role="alert">{{ error }}</div>

    <textarea
      class="snapshot-content-textarea"
      :value="modelValue"
      :disabled="!snapshotId || loading || saving"
      spellcheck="false"
      @input="onInput"
    ></textarea>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: string;
  snapshotId: string;
  loading?: boolean;
  saving?: boolean;
  dirty?: boolean;
  error?: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();

function onInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement;
  emit('update:modelValue', target.value);
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

.snapshot-editor-status {
  flex-shrink: 0;
  min-height: 38px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  background: #f8fafc;
}

.snapshot-editor-status p {
  margin: 0;
}

.snapshot-editor-status .dirty {
  color: #9a3412;
}

.snapshot-content-textarea {
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

.snapshot-content-textarea:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 32%, transparent);
  outline-offset: -2px;
}

.editor-error {
  margin: 10px 12px;
}
</style>
