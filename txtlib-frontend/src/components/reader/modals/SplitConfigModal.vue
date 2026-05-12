<template>
  <div v-if="open" class="modal-overlay" role="presentation" @click="onClose">
    <section class="panel split-modal" role="dialog" aria-modal="true" aria-labelledby="split-modal-title" @click.stop>
      <header class="split-header">
        <h3 id="split-modal-title">Reader Split Settings</h3>
        <button class="icon-close" type="button" aria-label="Close split dialog" :disabled="savingSplit" @click="onClose">
          ×
        </button>
      </header>

      <p class="split-desc">Apply section splitting without leaving reader. Current reading position will be preserved.</p>

      <div v-if="splitModalError" class="error" role="alert">{{ splitModalError }}</div>

      <form class="split-form" @submit.prevent="onSubmitSplitConfig">
        <label class="field">
          <span class="label">Split Type</span>
          <select v-model="draftType" class="input" :disabled="savingSplit">
            <option value="none">none</option>
            <option value="line_count">line_count</option>
            <option value="regex">regex</option>
            <option value="lines">lines</option>
          </select>
        </label>

        <label v-if="draftType === 'line_count'" class="field">
          <span class="label">line_count</span>
          <input
            v-model="draftLineCount"
            class="input"
            type="number"
            min="1"
            step="1"
            placeholder="e.g. 100"
            :disabled="savingSplit"
          />
        </label>

        <label v-if="draftType === 'regex'" class="field">
          <span class="label">regex</span>
          <textarea
            v-model="draftRegex"
            class="input split-textarea"
            rows="4"
            placeholder="e.g. ^Chapter\\s+\\d+"
            :disabled="savingSplit"
          />
        </label>

        <label v-if="draftType === 'lines'" class="field">
          <span class="label">lines (1-based, comma or space separated)</span>
          <textarea
            v-model="draftLines"
            class="input split-textarea"
            rows="4"
            placeholder="e.g. 1, 101, 201"
            :disabled="savingSplit"
          />
        </label>

        <div class="actions">
          <button class="button" type="button" :disabled="savingSplit" @click="onClose">Cancel</button>
          <button class="button primary" type="submit" :disabled="savingSplit">
            {{ savingSplit ? 'Saving...' : 'Save Split Config' }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import type { SplitConfig, SplitType } from '../../../types/book';

const props = defineProps<{
  open: boolean;
  splitConfig: SplitConfig;
}>();

const emit = defineEmits<{
  close: [];
  saved: [config: SplitConfig];
}>();

const savingSplit = ref(false);
const splitModalError = ref('');
const draftType = ref<SplitType>('none');
const draftLineCount = ref('100');
const draftRegex = ref('');
const draftLines = ref('1');

function hydrateSplitDraft(config: SplitConfig): void {
  draftType.value = config.type;
  draftLineCount.value = String(config.line_count ?? 100);
  draftRegex.value = config.regex ?? '';
  draftLines.value = (config.lines ?? []).join(', ');
}

function buildDraftSplitConfig(): SplitConfig {
  if (draftType.value === 'line_count') {
    const parsed = Number.parseInt(draftLineCount.value, 10);
    return {
      type: 'line_count',
      line_count: Number.isNaN(parsed) ? 0 : parsed
    };
  }

  if (draftType.value === 'regex') {
    return {
      type: 'regex',
      regex: draftRegex.value
    };
  }

  if (draftType.value === 'lines') {
    const lines = draftLines.value
      .split(/[\s,]+/)
      .map((token) => Number.parseInt(token, 10))
      .filter((num) => !Number.isNaN(num));

    return {
      type: 'lines',
      lines
    };
  }

  return { type: 'none' };
}

function onClose(): void {
  if (savingSplit.value) {
    return;
  }
  emit('close');
}

function onSubmitSplitConfig(): void {
  savingSplit.value = true;
  splitModalError.value = '';

  try {
    emit('saved', buildDraftSplitConfig());
  } catch (err) {
    splitModalError.value = err instanceof Error ? err.message : 'Failed to update split config';
  } finally {
    savingSplit.value = false;
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }
    hydrateSplitDraft(props.splitConfig);
    splitModalError.value = '';
  },
  { immediate: true }
);
</script>

<style scoped src="../../../styles/reader/reader-modal.css"></style>
