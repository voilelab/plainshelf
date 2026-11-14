<template>
  <BaseDialog :open="open" :title="t('bookCollection.selection.seriesTitle')" :busy="busy" @close="emit('cancel')">
    <form class="panel set-series-modal" novalidate @submit.prevent="submit">
      <h2>{{ t('bookCollection.selection.seriesTitle') }}</h2>
      <label>
        <span>{{ t('bookCollection.selection.seriesName') }}</span>
        <input
          v-model="name"
          class="input"
          type="text"
          name="series-name"
          :list="listId"
          :disabled="clear"
          :placeholder="t('bookCollection.selection.seriesNamePlaceholder')"
        />
      </label>
      <datalist :id="listId">
        <option v-for="option in knownNames" :key="option" :value="option" />
      </datalist>
      <label>
        <span>{{ t('bookCollection.selection.seriesStartIndex') }}</span>
        <input
          v-model="startInput"
          class="input"
          type="number"
          name="series-start-index"
          min="0"
          step="any"
          inputmode="decimal"
          :disabled="clear"
        />
      </label>
      <p class="set-series-help">{{ t('bookCollection.selection.seriesHelp') }}</p>
      <label class="set-series-clear">
        <input v-model="clear" type="checkbox" name="series-clear" />
        <span>{{ t('bookCollection.selection.seriesClear') }}</span>
      </label>
      <p v-if="problem" class="set-series-error" role="alert">{{ t(`bookCollection.selection.${problem}`) }}</p>
      <footer>
        <button type="button" class="button" :disabled="busy" @click="emit('cancel')">{{ t('common.cancel') }}</button>
        <button type="submit" class="button primary" :disabled="busy">
          {{ busy ? t('bookCollection.selection.processing') : confirmText }}
        </button>
      </footer>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import BaseDialog from '@/components/BaseDialog.vue';
import { useI18n } from '@/i18n';

export type SeriesBatchDraft = { name: string; startIndex: number } | null;

const props = defineProps<{
  open: boolean;
  count: number;
  knownNames: string[];
  busy?: boolean;
}>();
const emit = defineEmits<{ cancel: []; submit: [draft: SeriesBatchDraft] }>();
const { t } = useI18n();
const listId = useId();
const name = ref('');
const startInput = ref<string | number>('1');
const clear = ref(false);
const problem = ref<'' | 'seriesNameRequired' | 'seriesStartInvalid'>('');
const confirmText = computed(() => (
  props.count === 1
    ? t('bookCollection.selection.confirmSeriesOne')
    : t('bookCollection.selection.confirmSeries', { count: props.count })
));

watch(() => props.open, (open) => {
  if (!open) return;
  name.value = '';
  startInput.value = '1';
  clear.value = false;
  problem.value = '';
});
watch([name, startInput, clear], () => { problem.value = ''; });

function submit(): void {
  if (clear.value) {
    emit('submit', null);
    return;
  }
  const trimmed = name.value.trim();
  if (trimmed === '') {
    problem.value = 'seriesNameRequired';
    return;
  }
  const rawStart = String(startInput.value).trim();
  const startIndex = Number(rawStart);
  if (rawStart === '' || !Number.isFinite(startIndex) || startIndex < 0) {
    problem.value = 'seriesStartInvalid';
    return;
  }
  emit('submit', { name: trimmed, startIndex });
}
</script>

<style scoped>
.set-series-modal { display: grid; gap: 16px; max-width: 440px; padding: 18px; width: min(100%, 440px); }
.set-series-modal h2, .set-series-modal p { margin: 0; }
.set-series-modal label { display: grid; gap: 6px; }
.set-series-modal .set-series-clear { align-items: center; display: flex; gap: 8px; }
.set-series-modal footer { display: flex; gap: 8px; justify-content: flex-end; }
.set-series-help { color: var(--muted); font-size: 13px; }
.set-series-error { color: var(--danger-ink); font-size: 13px; }
</style>
