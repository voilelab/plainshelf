<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.logs.title') }}</h3>
    <div class="setting-item">
      <div>
        <label class="setting-label" :for="FIELD_ID">{{ t('settings.logRetention.label') }}</label>
        <p class="setting-description">{{ t('settings.logRetention.description') }}</p>
        <p class="setting-description setting-effect">
          {{
            value === 0
              ? t('settings.logRetention.keepsEverything')
              : t('settings.logRetention.deletesOlderThan', { days: value })
          }}
        </p>
      </div>
      <NumberFieldRoot
        :id="FIELD_ID"
        class="number-field setting-number-field"
        :min="0"
        :max="MAX_LOG_RETENTION_DAYS"
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
import { MAX_LOG_RETENTION_DAYS } from '@/features/settings/utils/settingsDraft';
import { INTEGER_FORMAT_OPTIONS } from '@/utils/numberField';
import '@/styles/numeric-controls.css';

const FIELD_ID = 'settings-log-retention-days';

const props = defineProps<{
  value: number;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [value: number];
}>();

const { t } = useI18n();

const decreaseLabel = computed(() =>
  t('common.decrease', { label: t('settings.logRetention.label') })
);
const increaseLabel = computed(() =>
  t('common.increase', { label: t('settings.logRetention.label') })
);

const draft = ref<number | null>(props.value);
watch(() => props.value, (value) => {
  draft.value = value;
});

// Reka UI 2.10.5 NumberField emits undefined for empty input; NumberFieldInput refreshes its DOM value when formatted textValue changes.
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

<style scoped>
.setting-effect {
  font-weight: 600;
}
</style>
