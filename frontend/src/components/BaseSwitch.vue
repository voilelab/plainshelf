<template>
  <SwitchRoot
    class="base-switch"
    :model-value="modelValue"
    :disabled="disabled"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <SwitchThumb class="base-switch-thumb" />
  </SwitchRoot>
</template>

<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui';

defineProps<{
  modelValue: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
}>();
</script>

<style scoped>
/* Reka UI ships no styles, so the track and the thumb are ours; only their
   state comes from the primitive's data-state. The root renders a <button>,
   which is why a caller labels it by id rather than wrapping it in a label. */
.base-switch {
  background: var(--border);
  border: 1px solid transparent;
  border-radius: 999px;
  cursor: pointer;
  display: inline-flex;
  /* The row around it is a flex container; the track must keep its size. */
  flex: none;
  height: 22px;
  padding: 2px;
  transition: background-color 140ms ease-out;
  width: 40px;
}

.base-switch[data-state='checked'] {
  background: var(--accent);
}

.base-switch:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.base-switch:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.base-switch-thumb {
  background: var(--surface, #fff);
  border-radius: 999px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.2);
  display: block;
  height: 16px;
  transition: transform 140ms ease-out;
  width: 16px;
}

/* 40 track - 2 border - 4 padding - 16 thumb = 18. */
.base-switch-thumb[data-state='checked'] {
  transform: translateX(18px);
}

@media (prefers-reduced-motion: reduce) {
  .base-switch,
  .base-switch-thumb {
    transition: none;
  }
}
</style>
