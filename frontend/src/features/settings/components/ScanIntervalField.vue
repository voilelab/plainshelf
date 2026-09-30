<template>
  <div class="scan-interval">
    <label class="scan-interval-label" :for="modeId">
      {{ t(labelKey) }}
    </label>
    <div class="scan-interval-controls">
      <select
        :id="modeId"
        v-model="mode"
        class="setting-select"
        :disabled="disabled"
        :data-testid="`${testidPrefix}-mode`"
      >
        <option value="default">{{ t('settings.shelves.scanIntervalModeDefault') }}</option>
        <option value="interval">{{ t('settings.shelves.scanIntervalModeEvery') }}</option>
        <option value="always">{{ t('settings.shelves.scanIntervalModeAlways') }}</option>
      </select>
      <template v-if="mode === 'interval'">
        <NumberFieldRoot
          class="number-field scan-interval-amount"
          :min="1"
          :max="maxAmount"
          :step="1"
          :format-options="INTEGER_FORMAT_OPTIONS"
          :model-value="draft"
          :disabled="disabled"
          @update:model-value="onAmountUpdate"
        >
          <NumberFieldDecrement class="number-field-step" :aria-label="decreaseLabel">−</NumberFieldDecrement>
          <NumberFieldInput
            class="number-field-input"
            :aria-label="amountLabel"
            :data-testid="`${testidPrefix}-amount`"
          />
          <NumberFieldIncrement class="number-field-step" :aria-label="increaseLabel">+</NumberFieldIncrement>
        </NumberFieldRoot>
        <select
          v-model="unit"
          class="setting-select"
          :disabled="disabled"
          :aria-label="t('settings.shelves.scanIntervalUnitLabel')"
          :data-testid="`${testidPrefix}-unit`"
        >
          <option value="s">{{ t('settings.shelves.scanIntervalUnitSeconds') }}</option>
          <option value="m">{{ t('settings.shelves.scanIntervalUnitMinutes') }}</option>
          <option value="h">{{ t('settings.shelves.scanIntervalUnitHours') }}</option>
        </select>
      </template>
    </div>
    <p class="scan-interval-help">{{ helpText }}</p>
    <p v-if="adjustedFrom" class="scan-interval-help scan-interval-adjusted" role="status">
      {{ t('settings.shelves.scanIntervalAdjusted', { value: adjustedFrom }) }}
    </p>
  </div>
</template>

<script setup lang="ts">
import {
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
  NumberFieldRoot
} from 'reka-ui';
import { computed, nextTick, ref, useId, watch } from 'vue';
import { useI18n } from '@/i18n';
import { INTEGER_FORMAT_OPTIONS } from '@/utils/numberField';
import {
  maxScanIntervalAmount,
  scanIntervalFromSelection,
  scanIntervalToSelection,
  type ScanIntervalMode,
  type ScanIntervalUnit
} from '@/features/settings/utils/scanInterval';
import '@/styles/numeric-controls.css';

// The scan-interval and book-check-interval fields are the same control — a
// mode/amount/unit selection over a Go duration string, defaulting off (empty)
// and offering an "always" (0s) mode — differing only in their label, help text
// and test ids. The scan-interval keys are the defaults so the original call
// site keeps its exact markup and copy.
const props = withDefaults(
  defineProps<{
    modelValue: string;
    disabled?: boolean;
    labelKey?: string;
    helpDefaultKey?: string;
    helpEveryKey?: string;
    helpAlwaysKey?: string;
    amountLabelKey?: string;
    testidPrefix?: string;
  }>(),
  {
    disabled: false,
    labelKey: 'settings.shelves.scanIntervalLabel',
    helpDefaultKey: 'settings.shelves.scanIntervalHelpDefault',
    helpEveryKey: 'settings.shelves.scanIntervalHelpEvery',
    helpAlwaysKey: 'settings.shelves.scanIntervalHelpAlways',
    amountLabelKey: 'settings.shelves.scanIntervalAmountLabel',
    testidPrefix: 'scan-interval'
  }
);
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

const { t } = useI18n();
const modeId = `scan-interval-mode-${useId()}`;

// The box and its two steppers name whichever interval this instance edits, so
// the per-book field does not announce itself as the shelf-wide scan setting.
const amountLabel = computed(() => t(props.amountLabelKey));
const decreaseLabel = computed(() => t('common.decrease', { label: amountLabel.value }));
const increaseLabel = computed(() => t('common.increase', { label: amountLabel.value }));

const mode = ref<ScanIntervalMode>('default');
const unit = ref<ScanIntervalUnit>('m');
// The committed amount, always whole and in range, so nothing the box is in the
// middle of can produce an invalid duration.
const amount = ref(1);
// What the box shows: the committed amount, except for the one tick that
// redraws it after the user empties it.
const draft = ref<number | null>(1);
const adjustedFrom = ref('');

// The largest amount that still fits Go's time.Duration in the chosen unit;
// above it time.ParseDuration fails and the raw error would be back.
const maxAmount = computed(() => maxScanIntervalAmount(unit.value));

function clampAmount(value: number): number {
  if (!Number.isFinite(value)) {
    return 1;
  }
  return Math.min(Math.max(1, Math.floor(value)), maxAmount.value);
}

function setAmount(value: number): void {
  amount.value = value;
  draft.value = value;
}

// reka commits on blur, Enter and the steppers rather than on every keystroke,
// and reports `undefined` for an emptied box. There is nothing to build a
// duration from then, so the committed amount stands and the box is redrawn
// with it. The null tick is what makes that visible: reka refreshes the input
// only when the formatted value changes, and it is already blank.
async function onAmountUpdate(next: number | undefined): Promise<void> {
  if (next === undefined) {
    draft.value = null;
    await nextTick();
    draft.value = amount.value;
    return;
  }
  setAmount(clampAmount(next));
}

// `maxScanIntervalAmount` falls as the unit grows, so an amount that fits in
// seconds can be past what hours can hold. reka clamps against `:max` only when
// it applies a value of its own, so the new ceiling has to be carried here;
// otherwise the box keeps showing a number the field would never save.
watch(unit, () => {
  const capped = clampAmount(amount.value);
  if (capped !== amount.value) {
    setAmount(capped);
  }
});

const helpText = computed(() => {
  if (mode.value === 'always') {
    return t(props.helpAlwaysKey);
  }
  if (mode.value === 'interval') {
    return t(props.helpEveryKey);
  }
  return t(props.helpDefaultKey);
});

const duration = computed(() =>
  scanIntervalFromSelection({ mode: mode.value, amount: amount.value, unit: unit.value })
);

// The value this field last sent up, so the round trip through the parent does
// not reload (and renormalize) the controls while they are being edited.
let emitted: string | null = null;
// What the controls read as immediately after a load, consumed by the first
// change they report, so the move a load itself causes is not mistaken for an
// edit by the user.
let loadedDuration: string | null = null;

function load(stored: string): void {
  const selection = scanIntervalToSelection(stored);
  mode.value = selection.mode;
  unit.value = selection.unit;
  setAmount(selection.amount);
  adjustedFrom.value = selection.adjustedFrom;

  // The controls carry a single unit, so a stored `60s` or `1h30m` shows as one
  // minute or ninety minutes. Push that reading straight back up, so what the
  // field shows is exactly what a save would write.
  const canonical = scanIntervalFromSelection(selection);
  loadedDuration = canonical;
  if (canonical !== stored) {
    emitted = canonical;
    emit('update:modelValue', canonical);
  }
}

watch(
  () => props.modelValue,
  (value) => {
    if (value !== emitted) {
      load(value);
    }
  },
  { immediate: true }
);

watch(duration, (value) => {
  if (loadedDuration !== null) {
    const fromLoad = value === loadedDuration;
    loadedDuration = null;
    if (fromLoad) {
      return;
    }
  }
  // Whatever the stored value could not express, the user has now overridden.
  adjustedFrom.value = '';
  if (value !== props.modelValue) {
    emitted = value;
    emit('update:modelValue', value);
  }
});
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.scan-interval {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.scan-interval-label {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.scan-interval-controls {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* Reka's NumberField brings its own box (numeric-controls.css); this matches it
   to the two selects beside it and keeps the three on one row inside the shelf
   modal, which the settings page's wider 160px number field would not do. */
.scan-interval-amount {
  --number-field-height: 35px;
  --number-field-padding: 8px;
  --number-field-step-width: 24px;

  flex: none;
  font-size: 13px;
  width: 120px;
}

.scan-interval-help {
  color: #64748b;
  font-size: 12px;
  margin: 0;
}

.scan-interval-adjusted {
  color: #b45309;
}
</style>
