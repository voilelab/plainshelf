<template>
  <div
    class="progress-bar"
    role="progressbar"
    :aria-valuenow="clamped"
    aria-valuemin="0"
    aria-valuemax="100"
    :aria-label="label"
  >
    <div class="progress-bar-fill" :class="{ indeterminate }" :style="fillStyle" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(
  defineProps<{
    value: number;
    label?: string;
    // indeterminate renders an animated bar for work whose size is not known yet.
    indeterminate?: boolean;
  }>(),
  {
    label: undefined,
    indeterminate: false
  }
);

const clamped = computed(() => {
  if (!Number.isFinite(props.value)) {
    return 0;
  }
  return Math.min(Math.max(props.value, 0), 100);
});

const fillStyle = computed(() =>
  props.indeterminate ? undefined : { width: `${clamped.value}%` }
);
</script>

<style scoped>
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

.progress-bar-fill.indeterminate {
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

  .progress-bar-fill.indeterminate {
    animation: none;
    width: 100%;
  }
}
</style>
