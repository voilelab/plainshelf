<template>
  <BaseDialog :open="open" :title="title" @close="emit('close')">
    <section class="panel font-license-modal">
      <header class="font-license-modal-header">
        <h2>{{ title }}</h2>
        <button class="font-license-modal-close" type="button" :aria-label="closeLabel" @click="emit('close')">
          ×
        </button>
      </header>

      <p v-if="loading" class="font-license-modal-status" role="status">{{ loadingLabel }}</p>
      <p v-else-if="error" class="font-license-modal-error" role="alert">{{ error }}</p>
      <pre v-else class="font-license-modal-text">{{ text }}</pre>

      <footer class="font-license-modal-actions">
        <button class="button" type="button" @click="emit('close')">{{ closeLabel }}</button>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import BaseDialog from '@/components/BaseDialog.vue';

defineProps<{
  open: boolean;
  title: string;
  text: string;
  loading: boolean;
  error: string;
  loadingLabel: string;
  closeLabel: string;
}>();

const emit = defineEmits<{
  close: [];
}>();
</script>

<style scoped>
.font-license-modal {
  display: grid;
  gap: 14px;
  max-height: min(720px, calc(100vh / var(--app-zoom, 1) - 32px));
  padding: 18px;
  width: min(680px, calc(100vw / var(--app-zoom, 1) - 32px));
}

.font-license-modal-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.font-license-modal-header h2 {
  font-size: 20px;
  line-height: 1.2;
  margin: 0;
}

.font-license-modal-close {
  align-items: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 20px;
  height: 32px;
  justify-content: center;
  line-height: 1;
  width: 32px;
}

.font-license-modal-status,
.font-license-modal-error {
  margin: 0;
}

.font-license-modal-error {
  color: #b91c1c;
}

.font-license-modal-text {
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  margin: 0;
  min-height: 220px;
  overflow: auto;
  padding: 14px;
  white-space: pre-wrap;
}

.font-license-modal-actions {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 520px) {
  .font-license-modal {
    padding: 16px;
  }

  .font-license-modal-actions .button {
    width: 100%;
  }
}
</style>
