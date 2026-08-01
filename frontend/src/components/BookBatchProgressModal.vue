<template>
  <BaseDialog :open="open" :title="title" :dismissible="finished" :busy="running" @close="close">
    <section class="panel batch-progress-modal">
      <h2>{{ title }}</h2>
      <p>{{ statusText }}</p>
      <ProgressBar :value="percentage" :label="t('bookCollection.selection.progressLabel')" />
      <p class="batch-progress-value">{{ Math.round(percentage) }}%</p>
      <ul v-if="failures.length" class="batch-failures">
        <li v-for="failure in failures" :key="failure.book_id">
          <strong>{{ titleById[failure.book_id] ?? failure.book_id }}</strong>
          — {{ failureMessage(failure.code) }}
        </li>
      </ul>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <footer>
        <button type="button" class="button" :disabled="running" @click="close">
          {{ t('bookCollection.selection.close') }}
        </button>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import BaseDialog from './BaseDialog.vue';
import ProgressBar from './ProgressBar.vue';
import { useBookBatchOperations } from '@/composables/useBookBatchOperations';
import { useI18n } from '@/i18n';
import type { BookBatchFailureCode } from '@/types/task';

const { t } = useI18n();
const { open, chain, status, percentage, error, titleById, running, finished, result, close } = useBookBatchOperations();
const failures = computed(() => result.value?.failures ?? []);
const title = computed(() => result.value?.operation === 'trash'
  ? t('bookCollection.selection.trashTitle')
  : t('bookCollection.selection.moveTitle'));
const statusText = computed(() => {
  if (error.value) return t('bookCollection.selection.failed');
  if (!finished.value) return t('bookCollection.selection.processing');
  const current = result.value;
  if (!current) return t('bookCollection.selection.failed');
  if (current.failures.length === 0) return t('bookCollection.selection.complete', { count: current.succeeded_ids.length });
  return t('bookCollection.selection.partial', { succeeded: current.succeeded_ids.length, failed: current.failures.length });
});

function failureMessage(code: BookBatchFailureCode): string {
  return t(`bookCollection.selection.failureCodes.${code}`);
}
</script>

<style scoped>
.batch-progress-modal { display: grid; gap: 12px; max-width: 520px; padding: 18px; width: min(100%, 520px); }
.batch-progress-modal h2, .batch-progress-modal p { margin: 0; }
.batch-progress-value { color: var(--muted); text-align: right; }
.batch-failures { margin: 0; max-height: 180px; overflow: auto; padding-left: 20px; }
.batch-progress-modal footer { display: flex; justify-content: flex-end; }
</style>
