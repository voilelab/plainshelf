<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.logs.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.logRetention.label') }}</div>
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
      <input
        class="setting-number"
        type="number"
        inputmode="numeric"
        min="0"
        :max="MAX_LOG_RETENTION_DAYS"
        step="1"
        :value="value"
        :disabled="disabled"
        @change="emit('change', $event)"
      />
    </label>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from '@/i18n';
import { MAX_LOG_RETENTION_DAYS } from '@/features/settings/utils/settingsDraft';

defineProps<{
  value: number;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [event: Event];
}>();

const { t } = useI18n();
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.setting-effect {
  font-weight: 600;
}
</style>
