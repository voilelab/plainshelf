<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.logs.title') }}</h3>
    <div class="setting-item">
      <div>
        <label class="setting-label" :for="FIELD_ID">{{ t('settings.logRetention.label') }}</label>
        <p class="setting-description">{{ t('settings.logRetention.description') }}</p>
        <!-- Deleting files is not something to leave a user guessing about, so
             the panel says in words what the current number does. -->
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
        :model-value="value"
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
import { computed } from 'vue';

import { useI18n } from '@/i18n';
import { MAX_LOG_RETENTION_DAYS } from '@/features/settings/utils/settingsDraft';
import { INTEGER_FORMAT_OPTIONS } from '@/utils/numberField';
import '@/styles/numeric-controls.css';

const FIELD_ID = 'settings-log-retention-days';

defineProps<{
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

// reka reports `undefined` for an emptied box: no window to save, and the field
// shows the stored value again once it loses focus.
function onUpdate(next: number | undefined): void {
  if (next === undefined) {
    return;
  }
  emit('change', next);
}
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.setting-effect {
  font-weight: 600;
}
</style>
