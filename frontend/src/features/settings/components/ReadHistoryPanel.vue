<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.readHistory.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.readHistoryLimit.label') }}</div>
        <p class="setting-description">{{ t('settings.readHistoryLimit.description') }}</p>
      </div>
      <input
        class="setting-number"
        type="number"
        inputmode="numeric"
        min="0"
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
