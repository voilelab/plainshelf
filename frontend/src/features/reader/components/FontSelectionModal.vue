<template>
  <BaseDialog :open="open" :title="t('reader.fontDialog.title')" @close="emit('close')">
    <section class="panel font-selection-modal">
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
  type ReaderFont
} from '@/features/reader/composables/useReaderSettings';
import { useI18n } from '@/i18n';

defineProps<{
  open: boolean;
  fontFamily: ReaderFont;
}>();

const emit = defineEmits<{
  close: [];
  select: [font: ReaderFont];
}>();

const { t } = useI18n();

const fontOptions = READER_FONT_OPTIONS.map((option) => ({
  ...option,
  labelKey: `reader.fontDialog.fonts.${option.id}.label`,
  descriptionKey: `reader.fontDialog.fonts.${option.id}.description`
}));

// RadioGroupRoot carries its model value as `unknown`, so narrow it back to a
// known font id before forwarding the selection.
function onSelect(value: unknown): void {
  const option = fontOptions.find((candidate) => candidate.id === value);
  if (option) {
    emit('select', option.id);
  }
}
</script>

<style scoped src="../styles/reader-modal.css"></style>
