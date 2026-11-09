<template>
  <BaseDialog :open="open" :title="t('reader.fontDialog.title')" @close="emit('close')">
    <section class="panel font-selection-modal reading-surface" :data-reader-theme="theme">
      <header class="font-selection-header">
        <div>
          <h3>{{ t('reader.fontDialog.title') }}</h3>
          <p>{{ t('reader.fontDialog.description') }}</p>
        </div>
        <button class="icon-close" type="button" :aria-label="t('reader.fontDialog.close')" @click="emit('close')">×</button>
      </header>

      <RadioGroupRoot
        class="font-selection-list"
        :model-value="fontFamily"
        :aria-label="t('reader.fontDialog.optionsLabel')"
        @update:model-value="onSelect"
      >
        <RadioGroupItem
          v-for="option in fontOptions"
          :key="option.id"
          class="font-selection-option"
          :value="option.id"
          :style="{ fontFamily: option.cssFamily }"
        >
          <span class="font-selection-option-copy">
            <strong>{{ t(option.labelKey) }}</strong>
            <span>{{ t(option.descriptionKey) }}</span>
            <span class="font-selection-sample" lang="zh-Hant">{{ t('reader.fontDialog.sample') }}</span>
          </span>
          <RadioGroupIndicator class="font-selection-check" aria-hidden="true">✓</RadioGroupIndicator>
        </RadioGroupItem>
      </RadioGroupRoot>

      <h4 class="reader-theme-heading">{{ t('reader.fontDialog.themeHeading') }}</h4>
      <RadioGroupRoot
        class="reader-theme-list"
        orientation="horizontal"
        :model-value="theme"
        :aria-label="t('reader.fontDialog.themeOptionsLabel')"
        @update:model-value="onSelectTheme"
      >
        <RadioGroupItem
          v-for="option in READER_THEMES"
          :key="option"
          class="reader-theme-option"
          :value="option"
        >
          <span class="reader-theme-swatch reading-surface" :data-reader-theme="option" aria-hidden="true">Aa</span>
          <span>{{ t(`reader.fontDialog.themes.${option}`) }}</span>
        </RadioGroupItem>
      </RadioGroupRoot>

      <button class="button font-selection-done" type="button" @click="emit('close')">
        {{ t('reader.fontDialog.done') }}
      </button>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { RadioGroupIndicator, RadioGroupItem, RadioGroupRoot } from 'reka-ui';
import BaseDialog from '@/components/BaseDialog.vue';
import {
  READER_FONT_OPTIONS,
  READER_THEMES,
  type ReaderFont,
  type ReaderTheme
} from '@/features/reader/composables/useReaderSettings';
import { useI18n } from '@/i18n';

defineProps<{
  open: boolean;
  fontFamily: ReaderFont;
  theme: ReaderTheme;
}>();

const emit = defineEmits<{
  close: [];
  select: [font: ReaderFont];
  selectTheme: [theme: ReaderTheme];
}>();

const { t } = useI18n();

const fontOptions = READER_FONT_OPTIONS.map((option) => ({
  ...option,
  labelKey: `reader.fontDialog.fonts.${option.id}.label`,
  descriptionKey: `reader.fontDialog.fonts.${option.id}.description`
}));

function onSelect(value: unknown): void {
  const option = fontOptions.find((candidate) => candidate.id === value);
  if (option) {
    emit('select', option.id);
  }
}

function onSelectTheme(value: unknown): void {
  const theme = READER_THEMES.find((candidate) => candidate === value);
  if (theme) {
    emit('selectTheme', theme);
  }
}
</script>

<style scoped src="../styles/reader-modal.css"></style>
