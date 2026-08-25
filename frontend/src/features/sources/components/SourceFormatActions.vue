<template>
  <div class="source-format-actions">
    <span class="format-badge">{{ format.toUpperCase() }}</span>
    <template v-if="format === 'txt'">
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'manual-md')">{{ t('sources.formatActions.manualMarkdown') }}</button>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'regex-md')">{{ t('sources.formatActions.regexMarkdown') }}</button>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'line-count-md')">{{ t('sources.formatActions.lineCountMarkdown') }}</button>
    </template>
    <template v-else>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'plain-text')">{{ t('sources.formatActions.plainText') }}</button>
      <span class="meta">{{ t('sources.formatActions.plainTextHelp') }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { SourceConversionKind } from '@/features/sources/components/SourceConversionModal.vue';
import { useI18n } from '@/i18n';

const { t } = useI18n();

defineProps<{
  format: 'txt' | 'md';
  disabled: boolean;
}>();

const emit = defineEmits<{
  convert: [kind: SourceConversionKind];
}>();
</script>

<style scoped>
.source-format-actions {
  min-height: 42px;
  padding: 7px 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--border);
  background: #fffdf7;
}

.format-badge {
  font-size: 12px;
  font-weight: 700;
  border-radius: 999px;
  padding: 3px 8px;
  background: #e0f2fe;
  color: #075985;
}

</style>
