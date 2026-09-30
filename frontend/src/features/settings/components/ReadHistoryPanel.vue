<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.readHistory.title') }}</h3>
    <div class="setting-item">
      <div>
        <label class="setting-label" :for="FIELD_ID">{{ t('settings.readHistoryLimit.label') }}</label>
        <p class="setting-description">{{ t('settings.readHistoryLimit.description') }}</p>
      </div>
      <NumberFieldRoot
        :id="FIELD_ID"
        class="number-field setting-number-field"
        :min="0"
        :step="1"
        :format-options="INTEGER_FORMAT_OPTIONS"
        :model-value="draft"
        :disabled="disabled"
        @update:model-value="onUpdate"
      >
        <NumberFieldDecrement class="number-field-step" :aria-label="decreaseLabel">−</NumberFieldDecrement>
        <NumberFieldInput class="number-field-input" />
        <NumberFieldIncrement class="number-field-step" :aria-label="increaseLabel">+</NumberFieldIncrement>
      </NumberFieldRoot>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
  NumberFieldRoot
} from 'reka-ui';
import { computed, nextTick, ref, watch } from 'vue';

import { useI18n } from '@/i18n';
import { INTEGER_FORMAT_OPTIONS } from '@/utils/numberField';
import '@/styles/numeric-controls.css';

const FIELD_ID = 'settings-read-history-limit';

const props = defineProps<{
  value: number;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [value: number];
}>();

const { t } = useI18n();

const decreaseLabel = computed(() =>
  t('common.decrease', { label: t('settings.readHistoryLimit.label') })
);
const increaseLabel = computed(() =>
  t('common.increase', { label: t('settings.readHistoryLimit.label') })
);

// What the field shows, which is the stored value except while it is being
// edited. Reka drives the input from this, so restoring it here is what redraws
// the box.
const draft = ref<number | null>(props.value);
watch(() => props.value, (value) => {
  draft.value = value;
});

// reka reports `undefined` for an emptied box: there is no limit to save, and
// leaving it blank would sit over a still-active setting — and the steppers,
// which read the box's text, would then start from the minimum instead of it.
// The null tick is what makes the restore visible: reka refreshes the input
// only when the formatted value changes, and it is already blank.
async function onUpdate(next: number | undefined): Promise<void> {
  if (next === undefined) {
    draft.value = null;
    await nextTick();
    draft.value = props.value;
    return;
  }
  draft.value = next;
  emit('change', next);
}
</script>

<style scoped src="../styles/settings-form.css"></style>
