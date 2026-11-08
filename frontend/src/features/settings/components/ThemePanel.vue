<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.theme.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.theme.label') }}</div>
        <p class="setting-description">{{ t('settings.theme.description') }}</p>
      </div>
      <SelectRoot :model-value="theme" @update:model-value="onSelect">
        <SelectTrigger class="setting-select select-trigger" data-testid="theme-select">
          <SelectValue>{{ t(themeLabelKeys[theme]) }}</SelectValue>
        </SelectTrigger>
        <SelectPortal>
          <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
            <SelectViewport>
              <SelectItem v-for="option in themeOptions" :key="option" class="reka-menu-item" :value="option">
                <SelectItemText>{{ t(themeLabelKeys[option]) }}</SelectItemText>
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
import { parseAppTheme, useAppTheme, type AppTheme } from '@/composables/useAppTheme';
import { useI18n } from '@/i18n';

const { t } = useI18n();
const { theme, setTheme } = useAppTheme();

const themeOptions: AppTheme[] = ['system', 'light', 'dark'];
const themeLabelKeys = {
  system: 'settings.theme.system',
  light: 'settings.theme.light',
  dark: 'settings.theme.dark'
} as const satisfies Record<AppTheme, string>;

function onSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    setTheme(parseAppTheme(value));
  }
}
</script>

<style scoped src="../styles/settings-form.css"></style>
