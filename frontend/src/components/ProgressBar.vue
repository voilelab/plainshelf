<template>
  <ProgressRoot
    class="progress-bar"
    :model-value="indeterminate ? null : clamped"
    :max="100"
    :get-value-label="valueLabel"
  >
    <ProgressIndicator class="progress-bar-fill" :style="fillStyle" />
  </ProgressRoot>
</template>

<script setup lang="ts">
import { ProgressIndicator, ProgressRoot } from 'reka-ui';
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

// ProgressRoot rejects non-finite or out-of-range values by falling back to the
// indeterminate state, so the value is clamped before it reaches the primitive.
const clamped = computed(() => {
  if (!Number.isFinite(props.value)) {
    return 0;
  }
  return Math.min(Math.max(props.value, 0), 100);
});

const fillStyle = computed(() =>
  props.indeterminate ? undefined : { width: `${clamped.value}%` }
);

// ProgressRoot always labels itself through getValueLabel; returning the label
// prop keeps the caller's description and omits the attribute when unset.
const valueLabel = () => props.label;
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
