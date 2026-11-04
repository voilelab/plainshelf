<template>
  <RadioGroupRoot
    :model-value="modelValue"
    class="shelf-source-modes"
    :disabled="disabled"
    :aria-label="label"
    @update:model-value="onSelect"
  >
    <RadioGroupItem
      v-for="option in options"
      :key="option.value"
      class="shelf-source-mode"
      :value="option.value"
      :data-testid="option.testId"
    >
      <RadioGroupIndicator class="shelf-source-mode-check" aria-hidden="true">●</RadioGroupIndicator>
      <span class="shelf-source-mode-copy">
        <strong>{{ option.label }}</strong>
        <span v-if="option.help" class="shelf-source-mode-help">{{ option.help }}</span>
      </span>
    </RadioGroupItem>
  </RadioGroupRoot>
</template>

<script setup lang="ts" generic="T extends string">
import { RadioGroupIndicator, RadioGroupItem, RadioGroupRoot, type AcceptableValue } from 'reka-ui';

const props = defineProps<{
  modelValue: T;
  options: ReadonlyArray<{ value: T; label: string; help?: string; testId?: string }>;
  label: string;
  disabled?: boolean;
}>();

const emit = defineEmits<{ (event: 'update:modelValue', value: T): void }>();

function onSelect(value: AcceptableValue): void {
  const match = props.options.find((option) => option.value === value);
  if (match) {
    emit('update:modelValue', match.value);
  }
}
</script>

<style scoped>
.shelf-source-modes {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.shelf-source-mode {
  align-items: flex-start;
  background: transparent;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: inherit;
  cursor: pointer;
  display: flex;
  font: inherit;
  gap: 8px;
  padding: 8px 10px;
  text-align: left;
  width: 100%;
}

.shelf-source-mode[data-state='checked'] {
  border-color: #64748b;
}

.shelf-source-mode:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-source-mode-check {
  color: #334155;
  font-size: 12px;
  line-height: 18px;
  min-width: 10px;
}

.shelf-source-mode-copy {
  display: flex;
  flex-direction: column;
  font-size: 13px;
  gap: 2px;
}

.shelf-source-mode-help {
  color: #64748b;
  font-size: 12px;
}
</style>
