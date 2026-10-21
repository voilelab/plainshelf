<template>
  <nav class="mobile-tab-bar" :aria-label="t('layout.tabNavLabel')">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.to"
      :to="tab.to"
      class="mobile-tab"
      :class="{ active: isActive(tab) }"
    >
      <Icon :name="tab.icon" class="mobile-tab-icon" />
      <span class="mobile-tab-label">{{ t(tab.labelKey) }}</span>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router';
import Icon from '@/components/Icon.vue';
import type { IconName } from '@/components/icons/registry';
import { useI18n } from '@/i18n';

interface MobileTab {
  to: string;
  icon: IconName;
  labelKey: string;
  exact: boolean;
}

const tabs: readonly MobileTab[] = [
  { to: '/books', icon: 'library', labelKey: 'layout.library', exact: false },
  { to: '/read-history', icon: 'recently-read', labelKey: 'layout.recentlyRead', exact: true },
  { to: '/downloads', icon: 'downloads', labelKey: 'layout.downloads', exact: true },
  { to: '/settings', icon: 'settings', labelKey: 'layout.settings', exact: true }
];

const { t } = useI18n();
const route = useRoute();

function isActive(tab: MobileTab): boolean {
  if (tab.exact) {
    return route.path === tab.to;
  }
  return route.path === tab.to || route.path.startsWith(`${tab.to}/`);
}
</script>

<style scoped>
/* Fixed to the bottom of the viewport, below the drawer and its backdrop
   (z-index 40/41) so an open drawer covers it. Spans the gesture bar on the
   edge-to-edge Android shell, carrying the bottom inset itself so the labels
   stay clear of it. */
.mobile-tab-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 30;
  display: flex;
  background: rgba(255, 255, 255, 0.96);
  border-top: 1px solid var(--border);
  backdrop-filter: blur(8px);
  padding-bottom: env(safe-area-inset-bottom, 0px);
}

.mobile-tab {
  align-items: center;
  color: #64748b;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  justify-content: center;
  min-width: 0;
  padding: 8px 4px 6px;
  text-decoration: none;
}

.mobile-tab.active {
  color: #2563eb;
}

.mobile-tab-icon {
  height: 22px;
  width: 22px;
}

.mobile-tab-icon :deep(svg) {
  height: 22px;
  width: 22px;
}

.mobile-tab-label {
  font-size: 11px;
  line-height: 1.2;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
