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
          <SelectValue>{{ currentLabel }}</SelectValue>
        </SelectTrigger>
        <SelectPortal>
          <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
            <SelectViewport>
              <SelectItem v-for="mode in modes" :key="mode" class="reka-menu-item" :value="mode">
                <SelectItemText>{{ t(MODE_LABELS[mode]) }}</SelectItemText>
              </SelectItem>
            </SelectViewport>
          </SelectContent>
        </SelectPortal>
      </SelectRoot>
    </label>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
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
import { readerLaunchModes, type ReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { useI18n } from '@/i18n';

const props = defineProps<{
  value: ReaderLaunchMode;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [mode: ReaderLaunchMode];
}>();

const { t } = useI18n();

const MODE_LABELS = {
  'new-window': 'settings.readerLaunch.newWindow',
  'new-reader': 'settings.readerLaunch.newReader',
  'in-window': 'settings.readerLaunch.inWindow'
} as const satisfies Record<ReaderLaunchMode, string>;

const modes = readerLaunchModes();

// Reka UI 2.10.5 SelectItemText registers textContent on mount without refreshing the registry when locale text changes.
const currentLabel = computed(() => t(MODE_LABELS[props.value]));

function onSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    emit('change', value as ReaderLaunchMode);
  }
}
</script>

<style scoped src="../styles/settings-form.css"></style>
