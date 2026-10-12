<template>
  <TooltipProvider :delay-duration="300">
    <nav class="sidebar-rail-nav" :aria-label="t('layout.railNavLabel')">
      <TooltipRoot v-for="item in items" :key="item.to">
        <TooltipTrigger as-child>
          <RouterLink
            :to="item.to"
            class="sidebar-nav-item sidebar-rail-item"
            exact-active-class="active"
            :aria-label="item.label"
          >
            <SidebarNavIcon :name="item.icon" />
          </RouterLink>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
            {{ item.label }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>
    </nav>
  </TooltipProvider>
</template>

<script setup lang="ts">
import { TooltipContent, TooltipPortal, TooltipProvider, TooltipRoot, TooltipTrigger } from 'reka-ui';
import SidebarNavIcon from '@/components/SidebarNavIcon.vue';
import type { SidebarNavIconName } from '@/types/sidebarNavIcon';
import { useI18n } from '@/i18n';

export interface SidebarRailItem {
  to: string;
  icon: SidebarNavIconName;
  label: string;
}

defineProps<{
  items: SidebarRailItem[];
}>();

const { t } = useI18n();
</script>

<style scoped>
.sidebar-rail-nav {
  align-items: center;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 4px 8px;
}

.sidebar-rail-item {
  height: 32px;
  justify-content: center;
  padding: 0;
  width: 32px;
}

.sidebar-rail-item :deep(.sidebar-nav-icon) {
  margin-right: 0;
}
</style>
