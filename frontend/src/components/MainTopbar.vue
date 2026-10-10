<template>
  <header class="topbar">
    <div class="topbar-left">
      <button
        v-if="narrow"
        class="menu-btn"
        type="button"
        :aria-label="t(drawerOpen ? 'layout.closeMenu' : 'layout.openMenu')"
        :aria-expanded="drawerOpen"
        @click="emit('toggle-drawer')"
      >
        <Icon name="menu" />
      </button>
      <h1 class="brand">
        <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
        <span class="brand-name">{{ t('app.name') }}</span>
      </h1>
      <!-- On a narrow viewport the brand collapses to its icon and this
           takes the freed space to answer "where am I" — the current folder
           or page — which the full sidebar otherwise carries on wide. -->
      <span
        v-if="narrow && locationLabel"
        class="topbar-location"
        :title="locationLabel"
      >{{ locationLabel }}</span>
      <nav
        v-if="showHistoryControls"
        class="history-controls"
        :aria-label="t('layout.desktopHistoryNavigation')"
      >
        <button type="button" class="history-btn" :aria-label="t('layout.previousPage')" @click="goToPreviousPage">
          ←
        </button>
        <button type="button" class="history-btn" :aria-label="t('layout.nextPage')" @click="goToNextPage">
          →
        </button>
      </nav>
    </div>
    <div class="topbar-controls">
      <label class="language-select">
        <span>{{ t('language.label') }}</span>
        <SelectRoot :model-value="locale" @update:model-value="onLocaleSelect">
          <SelectTrigger class="button language-select-control">
            <SelectValue />
          </SelectTrigger>
          <SelectPortal>
            <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
              <SelectViewport>
                <SelectItem v-for="lang in supportedLocales" :key="lang" class="reka-menu-item" :value="lang">
                  <SelectItemText>{{ t(localeLabelKeyMap[lang]) }}</SelectItemText>
                </SelectItem>
              </SelectViewport>
            </SelectContent>
          </SelectPortal>
        </SelectRoot>
      </label>
    </div>
  </header>
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
import Icon from '@/components/Icon.vue';
import { isWailsRuntime } from '@/providers';
import appIcon from '@/assets/icon-192.png';
import { useI18n } from '@/i18n';

defineProps<{
  narrow: boolean;
  drawerOpen: boolean;
  locationLabel: string;
}>();

const emit = defineEmits<{
  'toggle-drawer': [];
}>();

const { locale, setLocale, supportedLocales, t } = useI18n();

// The Wails desktop shell has a browser-history stack worth navigating; the web
// and mobile clients don't surface these pills. Scoping this to MainLayout keeps
// them off the immersive ReaderLayout routes, where the keyboard ←/→ already
// mean previous/next chapter.
const showHistoryControls = computed(() => isWailsRuntime());

const localeLabelKeyMap: Record<(typeof supportedLocales)[number], 'language.en' | 'language.zhHant'> = {
  en: 'language.en',
  'zh-Hant': 'language.zhHant'
};

function goToPreviousPage(): void {
  window.history.back();
}

function goToNextPage(): void {
  window.history.forward();
}

function onLocaleSelect(value: AcceptableValue): void {
  if (typeof value !== 'string') {
    return;
  }

  if (supportedLocales.includes(value as (typeof supportedLocales)[number])) {
    setLocale(value as (typeof supportedLocales)[number]);
  }
}
</script>

<style scoped>
/* Sticks to the top of the viewport, so on the Android shell — which targets
   SDK 36, past the SDK 35 cutoff where edge-to-edge became mandatory — it sits
   under the status bar unless it carries the top inset itself. Insets are 0
   everywhere else, leaving the padding unchanged. */
.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  background: rgba(255, 255, 255, 0.92);
  border-bottom: 1px solid var(--border);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(14px + env(safe-area-inset-top, 0px)) calc(24px + env(safe-area-inset-right, 0px))
    14px calc(24px + env(safe-area-inset-left, 0px));
}

.topbar-left {
  align-items: center;
  display: inline-flex;
  gap: 10px;
  min-width: 0;
}

.menu-btn {
  align-items: center;
  background: #f6f9fc;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: #3e4e66;
  cursor: pointer;
  display: flex;
  font-size: 16px;
  height: 34px;
  justify-content: center;
  width: 38px;
}

.menu-btn svg {
  height: 18px;
  width: 18px;
}

.history-controls {
  align-items: center;
  display: inline-flex;
  gap: 6px;
}

.history-btn {
  align-items: center;
  background: #f6f9fc;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: #3e4e66;
  cursor: pointer;
  display: flex;
  font-size: 16px;
  height: 34px;
  justify-content: center;
  line-height: 1;
  width: 38px;
}

.history-btn:hover {
  background: #ecf2f9;
}

.topbar-controls {
  display: inline-flex;
  gap: 10px;
}

.language-select {
  align-items: center;
  display: inline-flex;
  gap: 8px;
}

.language-select span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.language-select-control {
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 13px;
  min-height: 32px;
  padding: 0 8px;
}

.brand {
  align-items: center;
  display: inline-flex;
  gap: 8px;
  margin: 0;
  font-size: 20px;
  letter-spacing: 0.3px;
}

.brand-icon {
  width: 20px;
  height: 20px;
  display: block;
}

.topbar-location {
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Keep in sync with NARROW_VIEWPORT_QUERY in composables/useViewport.ts. */
@media (max-width: 768px) {
  .topbar {
    padding: calc(10px + env(safe-area-inset-top, 0px)) calc(12px + env(safe-area-inset-right, 0px))
      10px calc(12px + env(safe-area-inset-left, 0px));
  }

  /* Language is a set-once preference; on a narrow screen it moves into the
     Settings page (its own tab) and the top bar spends that space on the
     brand-icon-plus-location pairing instead. The brand text collapses to the
     icon on a platform where the user already knows the app — but stays in the
     accessibility tree (visually hidden, not display:none) so the <h1> keeps a
     non-empty accessible name for screen readers. */
  .brand-name {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    padding: 0;
    border: 0;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
  }

  .topbar-controls {
    display: none;
  }
}
</style>
