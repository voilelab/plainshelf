<template>
  <div class="toolbar-bar char-count-bar">
    <label v-if="!hideLabel" class="toolbar-label" :for="MIN_INPUT_ID">{{ t('library.charCount.label') }}</label>
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
      <!-- The sweep it kicks off is tracked on the always-mounted library page
           (useContentStatsRefresh), so this control only starts it and renders
           the progress the page hands back; closing the panel does not abandon
           an in-flight sweep. -->
      <button
        v-if="!readOnly && unknownCount > 0"
        type="button"
        class="toolbar-control toolbar-button toolbar-regular char-count-refresh"
        :disabled="refreshRunning"
        @click="emit('refresh-stats')"
      >
        {{ refreshLabel }}
      </button>
      <span v-if="refreshOutcome" class="toolbar-label" role="status">{{ refreshOutcome }}</span>
      <span v-if="refreshError" class="toolbar-label char-count-error" role="alert">{{ refreshError }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
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
  /** Hide the built-in "Characters" label when a surrounding heading supplies it. */
  hideLabel?: boolean;
  /** Whether a recompute sweep is currently in flight (tracked by the page). */
  refreshRunning: boolean;
  /** The recompute button's current label ("Update statistics…" / "Updating…"). */
  refreshLabel: string;
  /** The settled outcome message, empty while running or before a first run. */
  refreshOutcome: string;
  /** A start/poll error from the sweep, empty when there is none. */
  refreshError: string;
}>();

const emit = defineEmits<{
  (event: 'update:range', range: CharCountRange): void;
  (event: 'refresh-stats'): void;
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
