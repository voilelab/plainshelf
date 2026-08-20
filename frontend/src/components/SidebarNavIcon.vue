<template>
  <span class="sidebar-nav-icon" aria-hidden="true">
    <Icon :name="name" />
  </span>
</template>

<script setup lang="ts">
import Icon from '@/components/Icon.vue';
import type { SidebarNavIconName } from '@/types/sidebarNavIcon';

defineProps<{
  name: SidebarNavIconName;
}>();
</script>

<style scoped>
.sidebar-nav-icon {
  color: #64748b;
  display: inline-flex;
  flex: 0 0 auto;
  height: 16px;
  margin-right: 8px;
  width: 16px;
}

.sidebar-nav-icon svg {
  display: block;
  height: 16px;
  width: 16px;
}

:global(.sidebar-nav-item.active) .sidebar-nav-icon {
  color: currentColor;
}
</style>
