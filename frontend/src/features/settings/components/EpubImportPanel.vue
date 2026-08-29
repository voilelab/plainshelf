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

    <!-- The switch renders a <button>, so this row is no longer one big
         <label>: the control is bound by id and named from the label element,
         which keeps clicking the text a toggle. -->
    <div class="setting-item">
      <div>
        <label :id="labelId" :for="switchId" class="setting-label">
          {{ t('settings.epubImport.includeDescriptionLabel') }}
        </label>
        <p :id="descriptionId" class="setting-description">
          {{ t('settings.epubImport.includeDescriptionHelp') }}
        </p>
      </div>
      <BaseSwitch
        :id="switchId"
        :model-value="includeDescription"
        :disabled="disabled"
        :aria-labelledby="labelId"
        :aria-describedby="descriptionId"
        @update:model-value="emit('update:includeDescription', $event)"
      />
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="split-config-actions">
      <button class="button primary" type="button" :disabled="disabled" @click="emit('save')">
        {{ saving ? t('settings.epubImport.saving') : t('settings.epubImport.save') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useId } from 'vue';
import BaseSwitch from '@/components/BaseSwitch.vue';
import type { EpubImportPreset } from '@/types/book';
import { useI18n } from '@/i18n';

defineProps<{
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

const switchId = `epub-include-description-${useId()}`;
const labelId = `epub-include-description-label-${useId()}`;
const descriptionId = `epub-include-description-help-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>
