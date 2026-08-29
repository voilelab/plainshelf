<template>
  <CheckboxRoot
    class="base-checkbox"
    :model-value="modelValue"
    :disabled="disabled"
    @update:model-value="emit('update:modelValue', $event === true)"
  >
    <CheckboxIndicator class="base-checkbox-indicator">
      <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
        <path d="M3.5 8.5 6.5 11.5 12.5 4.5" />
      </svg>
    </CheckboxIndicator>
  </CheckboxRoot>
</template>

<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui';

defineProps<{
  modelValue: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
}>();
</script>

<style scoped>
/* Reka ships no styles, so the box and the tick are ours; only their state
   comes from the primitive's data-state. The root renders a <button>, which is
   why a caller labels it by id rather than relying on a wrapping label alone.
   The tick lives in CheckboxIndicator, which renders nothing while unchecked -
   there is no hidden glyph to keep in sync with the value. */
.base-checkbox {
  align-items: center;
  background: var(--surface, #fff);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  display: inline-flex;
  /* Rows around these are flex containers; the box must keep its size. */
  flex: none;
  height: 16px;
  justify-content: center;
  padding: 0;
  transition: background-color 120ms ease-out, border-color 120ms ease-out;
  width: 16px;
}

.base-checkbox[data-state='checked'] {
  background: var(--accent);
  border-color: var(--accent);
}

.base-checkbox:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.base-checkbox:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.base-checkbox-indicator {
  align-items: center;
  display: flex;
  justify-content: center;
}

.base-checkbox-indicator svg {
  display: block;
  fill: none;
  height: 12px;
  stroke: #fff;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2.5;
  width: 12px;
}

@media (prefers-reduced-motion: reduce) {
  .base-checkbox {
    transition: none;
  }
}
</style>
