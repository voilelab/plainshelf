<template>
  <label class="field">
    <span class="label">{{ label }}</span>
    <input
      class="input"
      :value="modelValue"
      :placeholder="placeholder"
      @input="onInput"
    />
  </label>
</template>

<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    label: string;
    modelValue: string;
    placeholder?: string;
  }>(),
  {
    placeholder: ''
  }
);

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void;
}>();

function onInput(event: Event): void {
  const target = event.target as HTMLInputElement;
  emit('update:modelValue', target.value);
}
</script>

<style scoped>
.field {
  display: grid;
  gap: 6px;
}

.label {
  font-size: 13px;
  color: var(--muted);
}
</style>
