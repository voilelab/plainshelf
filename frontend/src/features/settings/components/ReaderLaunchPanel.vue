<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.readerLaunch.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.readerLaunch.label') }}</div>
        <p class="setting-description">{{ t('settings.readerLaunch.description') }}</p>
      </div>
      <select class="setting-select" :value="value" :disabled="disabled" @change="emit('change', $event)">
        <option value="new-reader">{{ t('settings.readerLaunch.newReader') }}</option>
        <option value="in-window">{{ t('settings.readerLaunch.inWindow') }}</option>
      </select>
    </label>
  </section>
</template>

<script setup lang="ts">
import type { ReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { useI18n } from '@/i18n';

defineProps<{
  value: ReaderLaunchMode;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [event: Event];
}>();

const { t } = useI18n();
</script>

<style scoped src="../styles/settings-form.css"></style>
