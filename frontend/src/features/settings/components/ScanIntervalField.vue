<template>
  <div class="scan-interval">
    <label class="scan-interval-label" :for="modeId">
      {{ t('settings.shelves.scanIntervalLabel') }}
    </label>
    <div class="scan-interval-controls">
      <select
        :id="modeId"
        v-model="mode"
        class="setting-select"
        :disabled="disabled"
        data-testid="scan-interval-mode"
      >
        <option value="default">{{ t('settings.shelves.scanIntervalModeDefault') }}</option>
        <option value="interval">{{ t('settings.shelves.scanIntervalModeEvery') }}</option>
        <option value="always">{{ t('settings.shelves.scanIntervalModeAlways') }}</option>
      </select>
      <template v-if="mode === 'interval'">
        <input
          v-model="amountText"
          class="setting-number scan-interval-amount"
          type="number"
          min="1"
          :max="maxAmount"
          step="1"
          inputmode="numeric"
          :disabled="disabled"
          :aria-label="t('settings.shelves.scanIntervalAmountLabel')"
          data-testid="scan-interval-amount"
          @blur="amountText = String(amount)"
        />
        <select
          v-model="unit"
          class="setting-select"
          :disabled="disabled"
          :aria-label="t('settings.shelves.scanIntervalUnitLabel')"
          data-testid="scan-interval-unit"
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
import { computed, ref, useId, watch } from 'vue';
import { useI18n } from '@/i18n';
import {
  maxScanIntervalAmount,
  scanIntervalFromSelection,
  scanIntervalToSelection,
  type ScanIntervalMode,
  type ScanIntervalUnit
} from '@/features/settings/utils/scanInterval';

const props = withDefaults(defineProps<{ modelValue: string; disabled?: boolean }>(), {
  disabled: false
});
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

const { t } = useI18n();
const modeId = `scan-interval-mode-${useId()}`;

const mode = ref<ScanIntervalMode>('default');
const unit = ref<ScanIntervalUnit>('m');
// Held as text so the box may be empty mid-typing; `amount` is what the value
// is actually built from, so no keystroke can produce an invalid duration.
const amountText = ref('1');
const adjustedFrom = ref('');

// The largest amount that still fits Go's time.Duration in the chosen unit;
// above it time.ParseDuration fails and the raw error would be back.
const maxAmount = computed(() => maxScanIntervalAmount(unit.value));

const amount = computed(() => {
  const parsed = Number.parseInt(amountText.value, 10);
  if (!Number.isFinite(parsed) || parsed < 1) {
    return 1;
  }
  return Math.min(parsed, maxAmount.value);
});

const helpText = computed(() => {
  if (mode.value === 'always') {
    return t('settings.shelves.scanIntervalHelpAlways');
  }
  if (mode.value === 'interval') {
    return t('settings.shelves.scanIntervalHelpEvery');
  }
  return t('settings.shelves.scanIntervalHelpDefault');
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
  amountText.value = String(selection.amount);
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

.scan-interval-amount {
  max-width: 90px;
  padding: 7px 10px;
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
