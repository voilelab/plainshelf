<template>
  <ProgressRoot
    v-slot="{ modelValue }"
    class="progress-bar"
    :model-value="value"
    :get-value-label="() => label"
  >
    <ProgressIndicator
      class="progress-bar-fill"
      :style="typeof modelValue === 'number' ? { width: `${modelValue}%` } : undefined"
    />
  </ProgressRoot>
</template>

<script setup lang="ts">
import { ProgressIndicator, ProgressRoot } from 'reka-ui';

defineProps<{
  value: number | null;
  label?: string;
}>();
</script>

<style scoped>
/* Reka UI ships no styles, so the track and fill are ours; only their state
   comes from the primitive's data-state. */
.progress-bar {
  background: var(--surface-muted, #e2e8f0);
  border-radius: 999px;
  height: 8px;
  overflow: hidden;
  width: 100%;
}

.progress-bar-fill {
  background: var(--accent, #2563eb);
  border-radius: inherit;
  height: 100%;
  transition: width 160ms ease-out;
  width: 0;
}

.progress-bar-fill[data-state='indeterminate'] {
  animation: progress-bar-slide 1.2s ease-in-out infinite;
  width: 40%;
}

@keyframes progress-bar-slide {
  0% {
    margin-inline-start: -40%;
  }
  100% {
    margin-inline-start: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .progress-bar-fill {
    transition: none;
  }

  .progress-bar-fill[data-state='indeterminate'] {
    animation: none;
    width: 100%;
  }
}
</style>
