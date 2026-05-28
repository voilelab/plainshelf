<template>
  <nav v-if="showDesktopHistoryControls" class="desktop-history-controls" :aria-label="t('app.desktopHistoryNavigation')">
    <button type="button" class="desktop-history-button" :aria-label="t('app.previousPage')" @click="goToPreviousPage">
      ←
    </button>
    <button type="button" class="desktop-history-button" :aria-label="t('app.nextPage')" @click="goToNextPage">
      →
    </button>
  </nav>
  <RouterView />
  <div v-if="showMockModeBadge" class="mock-mode-badge" role="status" aria-live="polite">
    {{ t('app.mockModeBadge') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { isMockApiMode } from './api/client';
import { useI18n } from './i18n';

const { t } = useI18n();
const showMockModeBadge = computed(() => isMockApiMode());
const showDesktopHistoryControls = computed(() => {
  if (typeof window === 'undefined') {
    return false;
  }

  const params = new URLSearchParams(window.location.search);
  return (
    window.location.protocol === 'wails:' ||
    window.location.host.endsWith('.wails.localhost') ||
    params.get('desktop-shell-preview') === '1'
  );
});

function goToPreviousPage(): void {
  window.history.back();
}

function goToNextPage(): void {
  window.history.forward();
}
</script>

<style scoped>
.desktop-history-controls {
  position: fixed;
  top: 4px;
  right: 200px;
  z-index: 1000;
  display: inline-flex;
  gap: 8px;
  padding: 6px;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(8px);
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.08);
}

.desktop-history-button {
  width: 32px;
  height: 32px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: #fff;
  color: var(--text);
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
}

.desktop-history-button:hover {
  background: #f8fafc;
}

.mock-mode-badge {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 1000;
  background: #7c2d12;
  color: #fff;
  border: 1px solid #9a3412;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  letter-spacing: 0.06em;
  font-weight: 700;
}
</style>
