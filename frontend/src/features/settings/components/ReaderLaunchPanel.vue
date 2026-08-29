<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.readerLaunch.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.readerLaunch.label') }}</div>
        <p class="setting-description">{{ t('settings.readerLaunch.description') }}</p>
      </div>
      <SelectRoot :model-value="value" :disabled="disabled" @update:model-value="onSelect">
        <SelectTrigger class="setting-select select-trigger">
          <SelectValue />
        </SelectTrigger>
        <SelectPortal>
          <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
            <SelectViewport>
              <SelectItem class="reka-menu-item" value="new-reader">
                <SelectItemText>{{ t('settings.readerLaunch.newReader') }}</SelectItemText>
              </SelectItem>
              <SelectItem class="reka-menu-item" value="in-window">
                <SelectItemText>{{ t('settings.readerLaunch.inWindow') }}</SelectItemText>
              </SelectItem>
            </SelectViewport>
          </SelectContent>
        </SelectPortal>
      </SelectRoot>
    </label>
  </section>
</template>

<script setup lang="ts">
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
import type { ReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { useI18n } from '@/i18n';

defineProps<{
  value: ReaderLaunchMode;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [mode: ReaderLaunchMode];
}>();

const { t } = useI18n();

// The setter in the parent coerces any unexpected option back to the default,
// so the raw cast mirrors the previous native-select behavior; the Select only
// ever yields one of the two declared values in practice.
function onSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    emit('change', value as ReaderLaunchMode);
  }
}
</script>

<style scoped src="../styles/settings-form.css"></style>
