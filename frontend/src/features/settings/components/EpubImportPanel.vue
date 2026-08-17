<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.epubImport.title') }}</h3>
    <p class="setting-description">{{ t('settings.epubImport.description') }}</p>

    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.epubImport.presetLabel') }}</div>
        <p class="setting-description">{{ t('settings.epubImport.presetHelp') }}</p>
      </div>
      <select
        class="setting-select"
        :value="preset"
        :disabled="disabled"
        @change="emit('update:preset', $event)"
      >
        <option value="markdown">{{ t('settings.epubImport.presetMarkdown') }}</option>
        <option value="plain">{{ t('settings.epubImport.presetPlain') }}</option>
      </select>
    </label>

    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.epubImport.includeDescriptionLabel') }}</div>
        <p class="setting-description">{{ t('settings.epubImport.includeDescriptionHelp') }}</p>
      </div>
      <input
        v-model="includeDescriptionModel"
        class="setting-checkbox"
        type="checkbox"
        :disabled="disabled"
      />
    </label>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="split-config-actions">
      <button class="button primary" type="button" :disabled="disabled" @click="emit('save')">
        {{ saving ? t('settings.epubImport.saving') : t('settings.epubImport.save') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { EpubImportPreset } from '@/types/book';
import { useI18n } from '@/i18n';

const props = defineProps<{
  preset: EpubImportPreset;
  includeDescription: boolean;
  error: string;
  disabled: boolean;
  saving: boolean;
}>();

const emit = defineEmits<{
  'update:preset': [event: Event];
  'update:includeDescription': [value: boolean];
  save: [];
}>();

const { t } = useI18n();

const includeDescriptionModel = computed({
  get: () => props.includeDescription,
  set: (value: boolean) => emit('update:includeDescription', value)
});
</script>

<style scoped src="../styles/settings-form.css"></style>
