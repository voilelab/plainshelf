<template>
  <RouterView />
  <div v-if="showMockModeBadge" class="mock-mode-badge" role="status" aria-live="polite">
    MOCK API MODE
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { isMockApiMode } from './api/client';

const showMockModeBadge = computed(() => isMockApiMode());
</script>

<style scoped>
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
