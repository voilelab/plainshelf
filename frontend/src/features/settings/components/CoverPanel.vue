<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.cover.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.coverToJpg.label') }}</div>
        <p class="setting-description">{{ t('settings.coverToJpg.description') }}</p>
      </div>
      <input
        class="setting-checkbox"
        type="checkbox"
        :checked="value"
        :disabled="disabled"
        @change="emit('change', $event)"
      />
    </label>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from '@/i18n';

defineProps<{
  value: boolean;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [event: Event];
}>();

const { t } = useI18n();
</script>

<style scoped src="../styles/settings-form.css"></style>
