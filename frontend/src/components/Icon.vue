<template>
  <component :is="ICONS[name]" class="app-icon" aria-hidden="true" focusable="false" />
</template>

<script setup lang="ts">
import { ICONS, type IconName } from '@/components/icons/registry';

defineProps<{
  name: IconName;
}>();
</script>

<style scoped>
.app-icon {
  display: block;
}

/* Tabler draws on a 24 grid at stroke-width 2; 1.8 keeps the lighter weight the
   hand-drawn icons had. The inlined SVG puts stroke-width on each path as a
   presentation attribute, which beats inheritance from the root <svg> — so the
   override has to reach the paths themselves, past this component's scope. */
.app-icon :deep([stroke-width]) {
  stroke-width: 1.8;
}
</style>
