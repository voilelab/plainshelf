<template>
  <nav class="mobile-tab-bar" :aria-label="t('layout.tabNavLabel')">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.to"
      :to="tab.to"
      class="mobile-tab"
      :active-class="tab.exact ? undefined : 'active'"
      :exact-active-class="tab.exact ? 'active' : undefined"
    >
      <Icon :name="tab.icon" class="mobile-tab-icon" />
      <span class="mobile-tab-label">{{ t(tab.labelKey) }}</span>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router';
import Icon from '@/components/Icon.vue';
import type { IconName } from '@/components/icons/registry';
import { useI18n } from '@/i18n';

// The mobile shell's primary navigation. It replaces the off-canvas drawer as
// the way to reach the frequent destinations, leaving the drawer for shelf
// switching and the folder tree (MainLayout hides the redundant nav sections
// on the mobile runtime).
//
// `library` matches `/books` and every book detail below it, so it is the one
// tab keyed on active-class (prefix match) rather than exact-active-class; the
// rest name a single leaf route and must not stay lit on their children.
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
