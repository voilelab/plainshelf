<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.epubImport.title') }}</h3>
    <p class="setting-description">{{ t('settings.epubImport.description') }}</p>

    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.epubImport.presetLabel') }}</div>
        <p class="setting-description">{{ t('settings.epubImport.presetHelp') }}</p>
      </div>
      <SelectRoot :model-value="preset" :disabled="disabled" @update:model-value="onPresetSelect">
        <SelectTrigger class="setting-select select-trigger">
          <SelectValue />
        </SelectTrigger>
        <SelectPortal>
          <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
            <SelectViewport>
              <SelectItem class="reka-menu-item" value="markdown">
                <SelectItemText>{{ t('settings.epubImport.presetMarkdown') }}</SelectItemText>
              </SelectItem>
              <SelectItem class="reka-menu-item" value="plain">
                <SelectItemText>{{ t('settings.epubImport.presetPlain') }}</SelectItemText>
              </SelectItem>
            </SelectViewport>
          </SelectContent>
        </SelectPortal>
      </SelectRoot>
    </label>

    <!-- Row-wide click target, same as the preset row above; see CoverPanel
         for why a <label> can still wrap the switch button. -->
    <label class="setting-item" :for="switchId">
      <div>
        <span :id="labelId" class="setting-label">
          {{ t('settings.epubImport.includeDescriptionLabel') }}
        </span>
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
import { useId } from 'vue';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
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
  'update:preset': [preset: EpubImportPreset];
  'update:includeDescription': [value: boolean];
  save: [];
}>();

const { t } = useI18n();

function onPresetSelect(value: AcceptableValue): void {
  if (value === 'markdown' || value === 'plain') {
    emit('update:preset', value);
  }
}

const switchId = `epub-include-description-${useId()}`;
const labelId = `epub-include-description-label-${useId()}`;
const descriptionId = `epub-include-description-help-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>
