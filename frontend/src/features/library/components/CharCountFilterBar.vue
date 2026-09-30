<template>
  <div class="toolbar-bar char-count-bar">
    <label v-if="!hideLabel" class="toolbar-label" :for="MIN_INPUT_ID">{{ t('library.charCount.label') }}</label>
    <NumberFieldRoot
      :id="MIN_INPUT_ID"
      class="number-field number-field-sm char-count-field"
      :min="0"
      :step="CHAR_COUNT_STEP"
      :step-snapping="false"
      :format-options="INTEGER_FORMAT_OPTIONS"
      :model-value="minValue"
      @update:model-value="onMinChange"
    >
      <NumberFieldDecrement class="number-field-step" :aria-label="t('common.decrease', { label: t('library.charCount.minLabel') })">−</NumberFieldDecrement>
      <NumberFieldInput
        class="number-field-input char-count-input"
        :placeholder="t('library.charCount.minPlaceholder')"
        :aria-label="t('library.charCount.minLabel')"
      />
      <NumberFieldIncrement class="number-field-step" :aria-label="t('common.increase', { label: t('library.charCount.minLabel') })">+</NumberFieldIncrement>
    </NumberFieldRoot>
    <span class="char-count-separator" aria-hidden="true">–</span>
    <NumberFieldRoot
      :id="MAX_INPUT_ID"
      class="number-field number-field-sm char-count-field"
      :min="0"
      :step="CHAR_COUNT_STEP"
      :step-snapping="false"
      :format-options="INTEGER_FORMAT_OPTIONS"
      :model-value="maxValue"
      @update:model-value="onMaxChange"
    >
      <NumberFieldDecrement class="number-field-step" :aria-label="t('common.decrease', { label: t('library.charCount.maxLabel') })">−</NumberFieldDecrement>
      <NumberFieldInput
        class="number-field-input char-count-input"
        :placeholder="t('library.charCount.maxPlaceholder')"
        :aria-label="t('library.charCount.maxLabel')"
      />
      <NumberFieldIncrement class="number-field-step" :aria-label="t('common.increase', { label: t('library.charCount.maxLabel') })">+</NumberFieldIncrement>
    </NumberFieldRoot>
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
import {
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
  NumberFieldRoot
} from 'reka-ui';
import { computed } from 'vue';
import {
  CHAR_COUNT_STEP,
  isCharCountRangeActive,
  parseCharCountRange,
  type CharCountRange
} from '@/utils/charCountFilter';
import { useI18n } from '@/i18n';
import { INTEGER_FORMAT_OPTIONS } from '@/utils/numberField';
import '@/styles/numeric-controls.css';
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
// `null`, not `undefined`, for an unset bound: reka latches a NumberField into
// uncontrolled mode when its *initial* model-value is `undefined`, after which
// the parent's range would stop driving it.
const minValue = computed(() => props.range.min ?? null);
const maxValue = computed(() => props.range.max ?? null);

function asRaw(value: number | null): string {
  return value === null ? '' : String(value);
}

// Committed on blur, Enter and the steppers rather than on every keystroke, so
// a partially typed number never filters the list or fills up history. That is
// also what makes parseCharCountRange's swap of reversed bounds safe. An
// emptied box arrives as `undefined`, which it reads as "no bound".
// `step-snapping` is off so only the steppers move in CHAR_COUNT_STEP: snapping
// would round a search for 74 characters up to 100.
function commit(minRaw: string, maxRaw: string): void {
  emit('update:range', parseCharCountRange(minRaw, maxRaw));
}

function onMinChange(value: number | undefined): void {
  commit(value === undefined ? '' : String(value), asRaw(maxValue.value));
}

function onMaxChange(value: number | undefined): void {
  commit(asRaw(minValue.value), value === undefined ? '' : String(value));
}

function onClear(): void {
  emit('update:range', {});
}
</script>

<style scoped>
.char-count-bar {
  flex-wrap: wrap;
}

.char-count-field {
  width: 128px;
}

.char-count-input {
  /* Centred so the two bounds read as a pair between their steppers. */
  text-align: center;
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
