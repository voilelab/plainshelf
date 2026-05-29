<template>
  <span class="sidebar-nav-icon" aria-hidden="true">
    <svg v-if="name === 'recently-read'" viewBox="0 0 24 24" fill="none">
      <path d="M4 12a8 8 0 1 0 2.35-5.66" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M4 5.5v4h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      <path d="M12 8v4l2.5 2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
    <svg v-else-if="name === 'duplicate-content'" viewBox="0 0 24 24" fill="none">
      <rect x="5" y="7" width="11" height="13" rx="2" stroke="currentColor" stroke-width="1.8" />
      <path d="M8 4h8a3 3 0 0 1 3 3v10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      <path d="M8.5 11h4M8.5 14.5h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    </svg>
    <svg v-else-if="name === 'missing-author'" viewBox="0 0 24 24" fill="none">
      <circle cx="10" cy="8" r="3" stroke="currentColor" stroke-width="1.8" />
      <path d="M4.5 18c.75-3.1 2.75-4.5 5.5-4.5 1.75 0 3.17.56 4.17 1.72" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M17.5 11.5c1.35 0 2.5.9 2.5 2.2 0 1.8-2.5 1.83-2.5 3.3" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M17.5 20h.01" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" />
    </svg>
    <svg v-else-if="name === 'missing-cover'" viewBox="0 0 24 24" fill="none">
      <rect x="5" y="4" width="14" height="16" rx="2" stroke="currentColor" stroke-width="1.8" />
      <path d="M8.5 8h7M8.5 12h5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M4 20 20 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    </svg>
    <svg v-else-if="name === 'missing-language'" viewBox="0 0 24 24" fill="none">
      <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="1.8" />
      <path d="M4 11h14M11 4c2 2.2 2 11.8 0 14M11 4c-2 2.2-2 11.8 0 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M18.5 15.5c1.18 0 2 .75 2 1.75 0 1.5-2 1.52-2 2.75" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M18.5 22h.01" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" />
    </svg>
    <svg v-else-if="name === 'trash'" viewBox="0 0 24 24" fill="none">
      <path d="M5 7h14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M9 7V5.8c0-.44.36-.8.8-.8h4.4c.44 0 .8.36.8.8V7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M7.5 7.8v9.4a1.8 1.8 0 0 0 1.8 1.8h5.4a1.8 1.8 0 0 0 1.8-1.8V7.8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      <path d="M10.2 10.2v5.6M13.8 10.2v5.6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    </svg>
    <svg v-else-if="name === 'logs'" viewBox="0 0 24 24" fill="none">
      <path d="M7 4.75h7l4 4V19a1.75 1.75 0 0 1-1.75 1.75h-9.5A1.75 1.75 0 0 1 5 19V6.5A1.75 1.75 0 0 1 6.75 4.75H7Z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
      <path d="M14 4.75V9h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      <path d="M8.5 12h7M8.5 15h7M8.5 18h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    </svg>
  </span>
</template>

<script setup lang="ts">
import type { SidebarNavIconName } from '../types/sidebarNavIcon';

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
