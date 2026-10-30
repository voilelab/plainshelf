<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.language.title') }}</h3>
    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.language.label') }}</div>
        <p class="setting-description">{{ t('settings.language.description') }}</p>
      </div>
      <SelectRoot :model-value="locale" @update:model-value="onSelect">
        <SelectTrigger class="setting-select select-trigger">
          <SelectValue>{{ currentLabel }}</SelectValue>
        </SelectTrigger>
        <SelectPortal>
          <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
            <SelectViewport>
              <SelectItem
                v-for="lang in supportedLocales"
                :key="lang"
                class="reka-menu-item"
                :value="lang"
              >
                <SelectItemText>{{ t(localeLabelKeyMap[lang]) }}</SelectItemText>
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
import { useI18n } from '@/i18n';

const { locale, setLocale, supportedLocales, t } = useI18n();

const localeLabelKeyMap: Record<(typeof supportedLocales)[number], 'language.en' | 'language.zhHant'> = {
  en: 'language.en',
  'zh-Hant': 'language.zhHant'
};

// Reka UI 2.10.5 SelectItemText registers textContent on mount without refreshing the registry when locale text changes.
const currentLabel = computed(() => t(localeLabelKeyMap[locale.value]));

function onSelect(value: AcceptableValue): void {
  if (typeof value !== 'string') {
    return;
  }

  if (supportedLocales.includes(value as (typeof supportedLocales)[number])) {
    setLocale(value as (typeof supportedLocales)[number]);
  }
}
</script>

<style scoped src="../styles/settings-form.css"></style>
