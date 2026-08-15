<template>
  <div class="toolbar-bar char-count-bar">
    <label class="toolbar-label" :for="MIN_INPUT_ID">{{ t('library.charCount.label') }}</label>
    <input
      :id="MIN_INPUT_ID"
      class="toolbar-control toolbar-input char-count-input"
      type="number"
      inputmode="numeric"
      min="0"
      :step="CHAR_COUNT_STEP"
      :value="minValue"
      :placeholder="t('library.charCount.minPlaceholder')"
      :aria-label="t('library.charCount.minLabel')"
      @change="onMinChange"
    />
    <span class="char-count-separator" aria-hidden="true">–</span>
    <input
      :id="MAX_INPUT_ID"
      class="toolbar-control toolbar-input char-count-input"
      type="number"
      inputmode="numeric"
      min="0"
      :step="CHAR_COUNT_STEP"
      :value="maxValue"
      :placeholder="t('library.charCount.maxPlaceholder')"
      :aria-label="t('library.charCount.maxLabel')"
      @change="onMaxChange"
    />
    <button
      v-if="active"
      type="button"
      class="toolbar-control toolbar-button toolbar-small char-count-clear"
      :aria-label="t('library.charCount.clear')"
      @click="onClear"
    >✕</button>

    <template v-if="active">
      <span v-if="unknownCount > 0" class="toolbar-label char-count-unknown">
        {{ t('library.charCount.unknownNote', { count: unknownCount }) }}
      </span>
      <button
        v-if="!readOnly && unknownCount > 0"
        type="button"
        class="toolbar-control toolbar-button toolbar-regular char-count-refresh"
        :disabled="refreshStatsRunning"
        @click="onRefreshStats"
      >
        {{ refreshStatsLabel }}
      </button>
      <span v-if="refreshStatsOutcome" class="toolbar-label" role="status">{{ refreshStatsOutcome }}</span>
      <span v-if="refreshStatsError" class="toolbar-label char-count-error" role="alert">{{ refreshStatsError }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { bookshelfWriter } from '@/providers';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { isRefreshContentStatsResult } from '@/types/task';
import {
  CHAR_COUNT_STEP,
  isCharCountRangeActive,
  parseCharCountRange,
  type CharCountRange
} from '@/utils/charCountFilter';
import { useI18n } from '@/i18n';
import '@/styles/toolbar-controls.css';

const MIN_INPUT_ID = 'books-char-count-min';
const MAX_INPUT_ID = 'books-char-count-max';

const props = defineProps<{
  range: CharCountRange;
  /** Books in the current result set whose character count could not be read. */
  unknownCount: number;
  readOnly: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:range', range: CharCountRange): void;
  (event: 'stats-refreshed'): void;
}>();

const { t } = useI18n();

const active = computed(() => isCharCountRangeActive(props.range));
const minValue = computed(() => (props.range.min === undefined ? '' : String(props.range.min)));
const maxValue = computed(() => (props.range.max === undefined ? '' : String(props.range.max)));

// Committed on change (blur, Enter, spinner) rather than on every keystroke, so
// a partially typed number never filters the list or fills up history. That is
// also what makes parseCharCountRange's swap of reversed bounds safe.
function commit(minRaw: string, maxRaw: string): void {
  emit('update:range', parseCharCountRange(minRaw, maxRaw));
}

function onMinChange(event: Event): void {
  commit((event.target as HTMLInputElement).value, maxValue.value);
}

function onMaxChange(event: Event): void {
  commit(minValue.value, (event.target as HTMLInputElement).value);
}

function onClear(): void {
  emit('update:range', {});
}

// Recomputing statistics is a shelf-wide background sweep, so the bar tracks
// the task chain the server hands back rather than calling per book.
const {
  percentage: refreshStatsPercentage,
  error: refreshStatsError,
  running: refreshStatsRunning,
  finished: refreshStatsFinished,
  chain: refreshStatsChain,
  status: refreshStatsStatus,
  start: startRefreshStats
} = useTaskChainProgress({
  onSettled: () => emit('stats-refreshed'),
  startFailedMessage: () => t('library.charCount.refreshStats.failed'),
  pollFailedMessage: () => t('library.charCount.refreshStats.failed')
});

const refreshStatsLabel = computed(() => {
  if (refreshStatsRunning.value) {
    return t('library.charCount.refreshStats.busy', {
      percent: Math.round(refreshStatsPercentage.value)
    });
  }

  return t('library.charCount.refreshStats.action', { count: props.unknownCount });
});

const refreshStatsResult = computed(() => {
  for (const task of refreshStatsChain.value?.tasks ?? []) {
    if (isRefreshContentStatsResult(task.result)) {
      return task.result;
    }
  }
  return null;
});

const refreshStatsOutcome = computed(() => {
  if (!refreshStatsFinished.value || refreshStatsError.value) {
    return '';
  }

  const result = refreshStatsResult.value;
  if (!result || refreshStatsStatus.value === 'failed') {
    return t('library.charCount.refreshStats.failed');
  }
  if (result.failures.length === 0) {
    return t('library.charCount.refreshStats.done', { count: result.refreshed });
  }
  return t('library.charCount.refreshStats.partial', {
    succeeded: result.refreshed,
    failed: result.failures.length
  });
});

async function onRefreshStats(): Promise<void> {
  if (props.readOnly) {
    return;
  }

  // A sweep already in flight returns its own ID, so this attaches to the
  // existing progress instead of scheduling a second one.
  await startRefreshStats(() => bookshelfWriter().refreshContentStats());
}
</script>

<style scoped>
.char-count-bar {
  flex-wrap: wrap;
}

.char-count-input {
  width: 84px;
}

.char-count-separator {
  color: var(--muted, #888);
}

.char-count-clear {
  color: var(--muted, #888);
  line-height: 1;
}

.char-count-clear:hover {
  color: var(--text, #333);
}

.char-count-refresh {
  white-space: nowrap;
}

.char-count-unknown {
  white-space: nowrap;
}

.char-count-error {
  color: var(--danger, #c0392b);
}
</style>
