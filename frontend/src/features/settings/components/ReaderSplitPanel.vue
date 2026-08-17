<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.defaultSplitConfig.label') }}</h3>
    <p class="setting-description">{{ t('settings.defaultSplitConfig.description') }}</p>

    <p v-if="error" class="settings-message settings-message-error" role="alert">
      {{ error }}
    </p>

    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.defaultSplitConfig.typeLabel') }}</div>
      </div>
      <select
        class="setting-select"
        :value="type"
        :disabled="disabled"
        @change="emit('update:type', $event)"
      >
        <option value="none">{{ t('settings.defaultSplitConfig.typeNone') }}</option>
        <option value="line_count">{{ t('settings.defaultSplitConfig.typeLineCount') }}</option>
        <option value="regex">{{ t('settings.defaultSplitConfig.typeRegex') }}</option>
      </select>
    </label>

    <label v-if="type === 'line_count'" class="setting-item setting-item-stacked">
      <div>
        <div class="setting-label">{{ t('settings.defaultSplitConfig.lineCountLabel') }}</div>
      </div>
      <input
        v-model="lineCountModel"
        class="setting-number"
        type="number"
        min="1"
        step="1"
        :placeholder="t('settings.defaultSplitConfig.lineCountPlaceholder')"
        :disabled="disabled"
      />
    </label>

    <div v-if="type === 'regex'" class="setting-item-stacked">
      <label>
        <div class="setting-label">{{ t('settings.defaultSplitConfig.regexLabel') }}</div>
        <p class="setting-description">{{ t('settings.defaultSplitConfig.regexHelp') }}</p>
      </label>
      <textarea
        v-model="regexModel"
        class="setting-textarea"
        rows="3"
        :placeholder="t('settings.defaultSplitConfig.regexPlaceholder')"
        :disabled="disabled"
      />
    </div>

    <div class="split-config-actions">
      <button class="button primary" type="button" :disabled="disabled" @click="emit('save')">
        {{ saving ? t('settings.defaultSplitConfig.saving') : t('settings.defaultSplitConfig.save') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { SplitType } from '@/types/book';
import { useI18n } from '@/i18n';

const props = defineProps<{
  type: SplitType;
  lineCount: string;
  regex: string;
  error: string;
  disabled: boolean;
  saving: boolean;
}>();

const emit = defineEmits<{
  'update:type': [event: Event];
  'update:lineCount': [value: string];
  'update:regex': [value: string];
  save: [];
}>();

const { t } = useI18n();

// Writable computeds keep `v-model` on the controls, so Vue still suppresses
// updates while an IME composition is in progress.
const lineCountModel = computed({
  get: () => props.lineCount,
  set: (value: string) => emit('update:lineCount', value)
});

const regexModel = computed({
  get: () => props.regex,
  set: (value: string) => emit('update:regex', value)
});
</script>

<style scoped src="../styles/settings-form.css"></style>
